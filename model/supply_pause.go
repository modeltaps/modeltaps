package model

import (
	"errors"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"

	"gorm.io/gorm/clause"
)

// MinPersistedPauseSeconds is the shortest pause that is stored and shared between nodes;
// shorter ones (such as the default 5-second rate-limit cooldown) stay in process memory.
const MinPersistedPauseSeconds = 60

// MaxPauseSeconds caps how far a failed recovery probe can stretch a pause.
const MaxPauseSeconds = 6 * 3600

// SupplyPause keeps a channel, or one model on a channel, out of rotation until Until. With
// Probe set it stays paused after Until until a test request succeeds. Model "" pauses the
// whole channel. Rows are removed when the pause ends.
type SupplyPause struct {
	Id        int    `json:"id"`
	ChannelId int    `json:"channel_id" gorm:"not null;uniqueIndex:idx_supply_pauses_channel_model,priority:1"`
	Model     string `json:"model" gorm:"type:varchar(255);not null;default:'';uniqueIndex:idx_supply_pauses_channel_model,priority:2"`
	Until     int64  `json:"until" gorm:"bigint;not null;index"`
	Seconds   int64  `json:"seconds" gorm:"bigint"`
	Rule      string `json:"rule" gorm:"type:varchar(64)"`
	Category  string `json:"category" gorm:"type:varchar(32)"`
	Reason    string `json:"reason" gorm:"type:text"`
	Probe     bool   `json:"probe"`
	Notify    bool   `json:"notify"`
	// Probes counts failed recovery probes; each one doubles Seconds.
	Probes    int   `json:"probes"`
	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (p *SupplyPause) TableName() string {
	return "supply_pauses"
}

// pauseIndex mirrors the supply_pauses table in memory for the balancer's hot path.
var pauseIndex = struct {
	sync.RWMutex
	byChannel map[int]map[string]*SupplyPause
}{byChannel: map[int]map[string]*SupplyPause{}}

func indexPause(p *SupplyPause) {
	pauseIndex.Lock()
	defer pauseIndex.Unlock()
	if pauseIndex.byChannel[p.ChannelId] == nil {
		pauseIndex.byChannel[p.ChannelId] = map[string]*SupplyPause{}
	}
	pauseIndex.byChannel[p.ChannelId][p.Model] = p
}

func unindexPause(channelId int, modelName string) {
	pauseIndex.Lock()
	defer pauseIndex.Unlock()
	delete(pauseIndex.byChannel[channelId], modelName)
	if len(pauseIndex.byChannel[channelId]) == 0 {
		delete(pauseIndex.byChannel, channelId)
	}
}

// PreIndexPause makes a pause take effect on this node before SavePause stores it.
func PreIndexPause(p *SupplyPause) {
	indexPause(p)
}

func pauseActive(p *SupplyPause, now int64) bool {
	return p != nil && (p.Until > now || p.Probe)
}

// IsPaused reports whether a stored pause keeps channelId from serving modelName: the whole
// channel is paused, or that model on it. A pause without a probe ends on time even before the
// recovery loop removes its row; one with a probe holds until a probe succeeds.
func IsPaused(channelId int, modelName string) bool {
	now := time.Now().Unix()
	pauseIndex.RLock()
	defer pauseIndex.RUnlock()
	models := pauseIndex.byChannel[channelId]
	if models == nil {
		return false
	}
	return pauseActive(models[""], now) || (modelName != "" && pauseActive(models[modelName], now))
}

// ReloadPauses replaces the in-memory index with the table, so pauses written by other nodes
// take effect here.
func ReloadPauses() error {
	var pauses []*SupplyPause
	if err := DB.Find(&pauses).Error; err != nil {
		return err
	}
	next := map[int]map[string]*SupplyPause{}
	for _, p := range pauses {
		if next[p.ChannelId] == nil {
			next[p.ChannelId] = map[string]*SupplyPause{}
		}
		next[p.ChannelId][p.Model] = p
	}
	pauseIndex.Lock()
	pauseIndex.byChannel = next
	pauseIndex.Unlock()
	return nil
}

// StartPauseSync reloads pauses from the database every interval.
func StartPauseSync(interval time.Duration) {
	for {
		time.Sleep(interval)
		if err := ReloadPauses(); err != nil {
			logger.SysError("reload supply pauses failed: " + err.Error())
		}
	}
}

// SavePause stores a pause, or extends an existing one for the same channel and model: a pause
// is never shortened. It reports whether a new pause started (false when one was extended),
// which is when a notice is due.
func SavePause(p *SupplyPause) (started bool, err error) {
	if DB == nil {
		return false, errors.New("database not ready")
	}
	var existing SupplyPause
	found := DB.Where("channel_id = ? AND model = ?", p.ChannelId, p.Model).Limit(1).Find(&existing).RowsAffected > 0
	if found {
		if p.Until <= existing.Until {
			indexPause(&existing)
			return false, nil
		}
		existing.Until, existing.Seconds, existing.Reason = p.Until, p.Seconds, p.Reason
		existing.Rule, existing.Category, existing.Probe, existing.Notify = p.Rule, p.Category, p.Probe, p.Notify
		if err := DB.Save(&existing).Error; err != nil {
			return false, err
		}
		indexPause(&existing)
		return false, nil
	}
	err = DB.Clauses(clause.OnConflict{DoNothing: true}).Create(p).Error
	if err != nil {
		return false, err
	}
	if p.Id == 0 {
		// Another node stored a pause for the same key in the meantime.
		return false, ReloadPauses()
	}
	indexPause(p)
	return true, nil
}

// ListPauses returns every stored pause, soonest end first.
func ListPauses() ([]*SupplyPause, error) {
	var pauses []*SupplyPause
	err := DB.Order("until asc, id asc").Find(&pauses).Error
	return pauses, err
}

// DuePauses returns the pauses whose time is up.
func DuePauses(now int64) ([]*SupplyPause, error) {
	var pauses []*SupplyPause
	err := DB.Where("until <= ?", now).Order("until asc").Find(&pauses).Error
	return pauses, err
}

// ResumePause removes a pause if it is still the one the caller saw (same id and Until), and
// reports whether this call removed it. With several nodes running recovery, one of them wins.
func ResumePause(p *SupplyPause) bool {
	result := DB.Where("id = ? AND until = ?", p.Id, p.Until).Delete(&SupplyPause{})
	if result.Error != nil {
		logger.SysError("resume supply pause failed: " + result.Error.Error())
		return false
	}
	if result.RowsAffected == 0 {
		return false
	}
	unindexPause(p.ChannelId, p.Model)
	return true
}

// ExtendPause pushes a pause after a failed probe: Seconds doubles up to MaxPauseSeconds. It
// reports whether this call extended it.
func ExtendPause(p *SupplyPause, now int64) bool {
	seconds := min(max(p.Seconds, 1)*2, MaxPauseSeconds)
	result := DB.Model(&SupplyPause{}).Where("id = ? AND until = ?", p.Id, p.Until).
		Updates(map[string]any{"until": now + seconds, "seconds": seconds, "probes": p.Probes + 1})
	if result.Error != nil || result.RowsAffected == 0 {
		return false
	}
	p.Until, p.Seconds, p.Probes = now+seconds, seconds, p.Probes+1
	indexPause(p)
	return true
}

// ResumePauseById removes a pause regardless of its time (manual resume).
func ResumePauseById(id int) (*SupplyPause, error) {
	var p SupplyPause
	if err := DB.First(&p, id).Error; err != nil {
		return nil, err
	}
	if err := DB.Delete(&SupplyPause{}, id).Error; err != nil {
		return nil, err
	}
	unindexPause(p.ChannelId, p.Model)
	return &p, nil
}

// ResumeChannelPausesInCategory removes a channel's pauses of one category (a balance refresh
// that finds money ends its quota pauses) and returns the ones removed.
func ResumeChannelPausesInCategory(channelId int, category string) ([]*SupplyPause, error) {
	var pauses []*SupplyPause
	if err := DB.Where("channel_id = ? AND category = ?", channelId, category).Find(&pauses).Error; err != nil {
		return nil, err
	}
	var resumed []*SupplyPause
	for _, p := range pauses {
		if ResumePause(p) {
			resumed = append(resumed, p)
		}
	}
	return resumed, nil
}
