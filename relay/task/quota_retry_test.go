package task

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/relay/task/base"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// SEC-55: the task submit flow keeps one reservation across the channel retries and settles it
// once. These tests drive relayTaskSubmit with a fake adaptor against a real Quota and an
// in-memory database, and check the balance after each attempt sequence.

const retryTestModel = "sec55-task-model"

var retryTestDBSeq atomic.Int64

func setupRetryTestDB(t *testing.T) {
	t.Helper()
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "task-retry-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatalf("init user token encoder: %v", err)
		}
	}
	dsn := fmt.Sprintf("file:task_retry_test_%d?mode=memory&cache=shared", retryTestDBSeq.Add(1))
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.LogDetail{},
		&model.Channel{}, &model.Task{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite := model.DB, common.UsingSQLite
	model.DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { model.DB, common.UsingSQLite = oldDB, oldSQLite })
}

func withRetryTestPrice(t *testing.T, price *model.Price) {
	t.Helper()
	old := model.PricingInstance
	model.PricingInstance = &model.Pricing{Prices: map[string]*model.Price{retryTestModel: price}}
	t.Cleanup(func() { model.PricingInstance = old })
}

func withRetryConfig(t *testing.T, retryTimes int) {
	t.Helper()
	oldRetry, oldCooldown := config.RetryTimes, config.RetryCooldownSeconds
	config.RetryTimes, config.RetryCooldownSeconds = retryTimes, 0
	t.Cleanup(func() { config.RetryTimes, config.RetryCooldownSeconds = oldRetry, oldCooldown })
}

type fakeTaskProvider struct {
	*providersBase.BaseProvider
}

func (p *fakeTaskProvider) GetRequestHeaders() map[string]string { return map[string]string{} }

// fakeTaskAdaptor answers each Relay call with the next entry of results (nil = success) and
// moves to the next channel on every SetProvider call.
type fakeTaskAdaptor struct {
	base.TaskBase
	channels []*model.Channel
	results  []*base.TaskError
	selected int
	relays   int
}

func (t *fakeTaskAdaptor) Init() *base.TaskError { return nil }

func (t *fakeTaskAdaptor) SetProvider() *base.TaskError {
	ch := t.channels[t.selected%len(t.channels)]
	t.selected++
	t.C.Set("channel_id", ch.Id)
	t.BaseProvider = &fakeTaskProvider{&providersBase.BaseProvider{Channel: ch, Context: t.C}}
	return nil
}

func (t *fakeTaskAdaptor) Relay() *base.TaskError {
	result := t.results[t.relays]
	t.relays++
	if result != nil {
		return result
	}
	t.InitTask()
	t.Task.TaskID = fmt.Sprintf("task-%d", t.relays)
	t.Task.ChannelId = t.BaseProvider.GetChannel().Id
	t.Response = map[string]string{"task_id": t.Task.TaskID}
	return nil
}

func (t *fakeTaskAdaptor) HandleError(err *base.TaskError) { t.C.JSON(err.StatusCode, err) }

func (t *fakeTaskAdaptor) ShouldRetry(_ *gin.Context, err *base.TaskError) bool {
	return err != nil && !err.LocalError
}

func (t *fakeTaskAdaptor) UpdateTaskStatus(_ context.Context, _ map[int][]string, _ map[string]*model.Task) error {
	return nil
}

func upstreamFailure() *base.TaskError {
	return base.StringTaskError(http.StatusInternalServerError, "submit_failed", "upstream rejected", false)
}

type retryFixture struct {
	userId, tokenId int
	adaptor         *fakeTaskAdaptor
	c               *gin.Context
}

const (
	retryUserQuota  = 100000
	retryTokenQuota = 50000
)

// newRetryFixture allows retryTimes retries after the first attempt; results answers each attempt.
func newRetryFixture(t *testing.T, retryTimes int, results ...*base.TaskError) *retryFixture {
	t.Helper()
	setupRetryTestDB(t)
	withRetryConfig(t, retryTimes)

	channels := []*model.Channel{taskTestChannel(9601), taskTestChannel(9602), taskTestChannel(9603)}
	fixture := map[int]*model.Channel{}
	for _, ch := range channels {
		fixture[ch.Id] = ch
		if err := model.DB.Create(&model.Channel{Id: ch.Id, Name: "ch"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	withTaskRuleFixture(t, fixture)

	userId := 55
	user := &model.User{Id: userId, Username: fmt.Sprintf("task-retry-%d", retryTestDBSeq.Load()),
		Password: "placeholder", Quota: retryUserQuota, Group: "g", AffCode: fmt.Sprintf("tr%d", retryTestDBSeq.Load())}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	token := &model.Token{UserId: userId, Name: "task-retry-token", RemainQuota: retryTokenQuota}
	if err := model.DB.Create(token).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/suno/submit/music", nil)
	c.Set("id", userId)
	c.Set("token_id", token.Id)
	c.Set("token_group", "g")
	c.Set("routing_model", tmRouting)
	c.Set("group_ratio", 1.0)

	adaptor := &fakeTaskAdaptor{
		TaskBase: base.TaskBase{C: c, Platform: model.TaskPlatformSuno, ModelName: retryTestModel},
		channels: channels,
		results:  results,
	}
	return &retryFixture{userId: userId, tokenId: token.Id, adaptor: adaptor, c: c}
}

func (f *retryFixture) run(t *testing.T) {
	t.Helper()
	relayTaskSubmit(f.c, f.adaptor)
	if !common.WaitTrackedGoroutines(5 * time.Second) {
		t.Fatal("task settlement did not finish")
	}
	if f.adaptor.relays != len(f.adaptor.results) {
		t.Fatalf("relay attempts = %d, want %d", f.adaptor.relays, len(f.adaptor.results))
	}
}

func (f *retryFixture) assertBalance(t *testing.T, charged int) {
	t.Helper()
	userQuota, err := model.GetUserQuota(f.userId)
	if err != nil {
		t.Fatal(err)
	}
	if userQuota != retryUserQuota-charged {
		t.Fatalf("user quota = %d, want %d (charged %d)", userQuota, retryUserQuota-charged, charged)
	}
	token, err := model.GetTokenById(f.tokenId)
	if err != nil {
		t.Fatal(err)
	}
	if token.RemainQuota != retryTokenQuota-charged {
		t.Fatalf("token remain = %d, want %d (charged %d)", token.RemainQuota, retryTokenQuota-charged, charged)
	}
}

func (f *retryFixture) tasks(t *testing.T) []model.Task {
	t.Helper()
	var tasks []model.Task
	if err := model.DB.Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	return tasks
}

// timesPrice charges 500 per task (1000 × 0.5).
func timesPrice() *model.Price {
	return &model.Price{Model: retryTestModel, Type: model.TimesPriceType, ChannelType: config.ChannelTypeSuno, Input: 0.5, Output: 0.5}
}

func TestTaskRetryFailFailSuccessChargesOnce(t *testing.T) {
	withRetryTestPrice(t, timesPrice())
	f := newRetryFixture(t, 2, upstreamFailure(), upstreamFailure(), nil)
	f.run(t)

	f.assertBalance(t, 500)
	tasks := f.tasks(t)
	if len(tasks) != 1 || tasks[0].Quota != 500 || tasks[0].ChannelId != 9603 {
		t.Fatalf("tasks = %+v, want one task on channel 9603 recording quota 500", tasks)
	}
}

func TestTaskRetryFailFailFailRefundsOnce(t *testing.T) {
	withRetryTestPrice(t, timesPrice())
	f := newRetryFixture(t, 2, upstreamFailure(), upstreamFailure(), upstreamFailure())
	f.run(t)

	f.assertBalance(t, 0)
	if tasks := f.tasks(t); len(tasks) != 0 {
		t.Fatalf("no task should be stored, got %+v", tasks)
	}
}

func TestTaskRetryStopConditionRefundsOnce(t *testing.T) {
	withRetryTestPrice(t, timesPrice())
	local := base.StringTaskError(http.StatusBadRequest, "invalid_request", "rejected locally", true)
	// Two retries are allowed, but the local error on the second attempt ends the loop.
	f := newRetryFixture(t, 2, upstreamFailure(), local)
	f.run(t)
	f.assertBalance(t, 0)
}

func TestTaskFirstAttemptSuccessChargesOnce(t *testing.T) {
	withRetryTestPrice(t, timesPrice())
	f := newRetryFixture(t, 2, nil)
	f.run(t)

	f.assertBalance(t, 500)
	if tasks := f.tasks(t); len(tasks) != 1 || tasks[0].Quota != 500 {
		t.Fatalf("tasks = %+v, want one task recording quota 500", tasks)
	}
}

// A per-token price reserves for the output estimate but settles one prompt token; task.Quota,
// which an async failure refunds, must be the settled amount and not 1000 × the input ratio.
func TestTaskQuotaRecordsChargedAmount(t *testing.T) {
	withRetryTestPrice(t, &model.Price{Model: retryTestModel, Type: model.TokensPriceType, ChannelType: config.ChannelTypeSuno, Input: 2, Output: 2})
	f := newRetryFixture(t, 1, upstreamFailure(), nil)
	f.run(t)

	f.assertBalance(t, 2)
	if tasks := f.tasks(t); len(tasks) != 1 || tasks[0].Quota != 2 {
		t.Fatalf("tasks = %+v, want one task recording quota 2", tasks)
	}
}
