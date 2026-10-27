package model

import (
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupLogCleanupDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&Log{}, &LogDetail{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

// shrinkLogCleanup makes batches tiny and retries instant for the duration of a test.
func shrinkLogCleanup(t *testing.T, batchSize int) {
	t.Helper()
	oldBatch, oldInterval, oldFunc := logCleanupBatchSize, logCleanupRetryInterval, deleteOldLogBatchFunc
	logCleanupBatchSize = batchSize
	logCleanupRetryInterval = time.Millisecond
	t.Cleanup(func() {
		logCleanupBatchSize, logCleanupRetryInterval, deleteOldLogBatchFunc = oldBatch, oldInterval, oldFunc
	})
}

// A second trigger while a cleanup holds the flag returns ErrLogCleanupRunning without touching
// the database and without clearing the running cleanup's flag.
func TestDeleteOldLogs_RejectsWhenRunning(t *testing.T) {
	if !logCleanupRunning.CompareAndSwap(false, true) {
		t.Fatal("cleanup flag unexpectedly set before test")
	}
	defer logCleanupRunning.Store(false)

	if !IsLogCleanupRunning() {
		t.Fatal("IsLogCleanupRunning should report true while flag is held")
	}
	n, err := DeleteOldLogs("test", 0)
	if !errors.Is(err, ErrLogCleanupRunning) || n != 0 {
		t.Fatalf("DeleteOldLogs = (%d, %v), want (0, ErrLogCleanupRunning)", n, err)
	}
	if err := StartLogCleanup("test", 0); !errors.Is(err, ErrLogCleanupRunning) {
		t.Fatalf("StartLogCleanup = %v, want ErrLogCleanupRunning", err)
	}
	if !IsLogCleanupRunning() {
		t.Fatal("rejected call must not clear the flag held by the running cleanup")
	}
}

// Old consume logs are deleted across several batches together with their log_details; newer
// logs, other log types and their details stay.
func TestDeleteOldLogs_BatchesAndCascadesDetails(t *testing.T) {
	setupLogCleanupDB(t)
	shrinkLogCleanup(t, 3)

	var oldIds, keptIds []int
	for i := 0; i < 7; i++ {
		l := &Log{Type: LogTypeConsume, CreatedAt: 100 + int64(i)}
		DB.Create(l)
		oldIds = append(oldIds, l.Id)
	}
	for _, l := range []*Log{
		{Type: LogTypeConsume, CreatedAt: 1000},
		{Type: LogTypeTopup, CreatedAt: 100},
		{Type: LogTypeSystem, CreatedAt: 100},
	} {
		DB.Create(l)
		keptIds = append(keptIds, l.Id)
	}
	for _, id := range append(append([]int{}, oldIds...), keptIds...) {
		DB.Create(&LogDetail{LogId: id})
	}

	n, err := DeleteOldLogs("test", 500)
	if err != nil || n != 7 {
		t.Fatalf("DeleteOldLogs = (%d, %v), want (7, nil)", n, err)
	}
	var logs, details int64
	DB.Model(&Log{}).Count(&logs)
	DB.Model(&LogDetail{}).Count(&details)
	if logs != int64(len(keptIds)) || details != int64(len(keptIds)) {
		t.Fatalf("left %d logs and %d details, want %d of each", logs, details, len(keptIds))
	}
	var orphan int64
	DB.Model(&LogDetail{}).Where("log_id IN ?", oldIds).Count(&orphan)
	if orphan != 0 {
		t.Fatalf("%d details of deleted logs remain", orphan)
	}
	if IsLogCleanupRunning() {
		t.Fatal("flag must be released after the cleanup finishes")
	}
}

// A failing batch is retried and the cleanup goes on; failures in a row up to the limit make it
// give up with the error and release the flag.
func TestDeleteOldLogs_RetriesTransientErrors(t *testing.T) {
	shrinkLogCleanup(t, 10)
	transient := errors.New("lock wait timeout")

	calls := 0
	deleteOldLogBatchFunc = func(int64, int) (int64, error) {
		calls++
		switch calls {
		case 1, 2:
			return 0, transient
		case 3:
			return 10, nil
		default:
			return 4, nil
		}
	}
	n, err := DeleteOldLogs("test", 0)
	if err != nil || n != 14 || calls != 4 {
		t.Fatalf("DeleteOldLogs = (%d, %v) after %d calls, want (14, nil) after 4", n, err, calls)
	}

	calls = 0
	deleteOldLogBatchFunc = func(int64, int) (int64, error) {
		calls++
		if calls == 1 {
			return 10, nil
		}
		return 0, transient
	}
	n, err = DeleteOldLogs("test", 0)
	if !errors.Is(err, transient) || n != 10 || calls != 1+logCleanupMaxErrs {
		t.Fatalf("DeleteOldLogs = (%d, %v) after %d calls, want (10, transient) after %d", n, err, calls, 1+logCleanupMaxErrs)
	}
	if IsLogCleanupRunning() {
		t.Fatal("flag must be released after giving up")
	}
}

// Concurrent manual triggers start exactly one background cleanup; the others are told one is
// running. Once it finishes a new one can start.
func TestStartLogCleanup_OneAtATime(t *testing.T) {
	shrinkLogCleanup(t, 10)
	release := make(chan struct{})
	var runs sync.WaitGroup
	runs.Add(1)
	deleteOldLogBatchFunc = func(int64, int) (int64, error) {
		<-release
		runs.Done()
		return 0, nil
	}

	var started, rejected int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := StartLogCleanup("test", 0)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				started++
			case errors.Is(err, ErrLogCleanupRunning):
				rejected++
			default:
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if started != 1 || rejected != 19 {
		t.Fatalf("started %d, rejected %d, want 1 and 19", started, rejected)
	}
	if !IsLogCleanupRunning() {
		t.Fatal("background cleanup should hold the flag until it finishes")
	}
	close(release)
	runs.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for IsLogCleanupRunning() {
		if time.Now().After(deadline) {
			t.Fatal("flag not released after the background cleanup finished")
		}
		time.Sleep(time.Millisecond)
	}
	deleteOldLogBatchFunc = func(int64, int) (int64, error) { return 0, nil }
	if err := StartLogCleanup("test", 0); err != nil {
		t.Fatalf("StartLogCleanup after finish = %v, want nil", err)
	}
	for IsLogCleanupRunning() {
		time.Sleep(time.Millisecond)
	}
}
