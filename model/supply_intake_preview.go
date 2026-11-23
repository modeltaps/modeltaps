package model

import (
	"sort"
	"strings"
)

// Trying intake rules before they are saved: what they would do to every public model of the
// catalog if a channel's upstream started listing it, and to one name typed in.

// IntakePreviewRule is how many catalog models a rule decides, with a few of their names.
// Index counts the rules from 0; -1 is the fallback for models no rule matches.
type IntakePreviewRule struct {
	Index    int      `json:"index"`
	Action   string   `json:"action"`
	Count    int      `json:"count"`
	Examples []string `json:"examples"`
}

// IntakePreviewModel is what the rules do to one model.
type IntakePreviewModel struct {
	Model    string `json:"model"`
	Known    bool   `json:"known"`
	Supplied bool   `json:"supplied"`
	Rule     int    `json:"rule"`
	Action   string `json:"action"`
	JoinLast bool   `json:"join_last"`
}

// IntakePreviewListed is one catalog model and the rule that decides it (-1 for the fallback).
type IntakePreviewListed struct {
	Model string `json:"model"`
	Rule  int    `json:"rule"`
}

// IntakePreview is the outcome of a set of rules over the catalog.
type IntakePreview struct {
	Total int                 `json:"total"`
	Rules []IntakePreviewRule `json:"rules"`
	Model *IntakePreviewModel `json:"model,omitempty"`
	// Models lists every catalog model with its rule, when asked for (a picker to try them).
	Models []IntakePreviewListed `json:"models,omitempty"`
}

const intakePreviewExamples = 3

func decideIntakeIndex(rules []SupplyIntakeRule, s intakeSubject) (int, SupplyIntakeRule) {
	for i, r := range rules {
		if r.matches(s) {
			return i, r
		}
	}
	return -1, fallbackIntake
}

// PreviewIntakeRules runs the rules over the public models of the catalog. A model counts as
// supplied when it has an enabled supply, as it would for a new channel's upstream listing it.
// With list, every model is returned with its rule. With name set, the outcome for that name is
// reported too; a name outside the catalog is judged
// by its name alone (no endpoints or vendor, not supplied).
func PreviewIntakeRules(rules []SupplyIntakeRule, name string, list bool) (*IntakePreview, error) {
	var infos []*ModelInfo
	if err := DB.Select("model", "endpoints", "vendor_id").
		Where("kind = ? AND (alias_of = ? OR alias_of IS NULL)", "", "").Order("model").Find(&infos).Error; err != nil {
		return nil, err
	}
	var supplied []string
	if err := DB.Model(&ModelSupply{}).Where("enabled = ? AND upstream_gone_at = ?", true, 0).Distinct().Pluck("model", &supplied).Error; err != nil {
		return nil, err
	}
	suppliedSet := make(map[string]bool, len(supplied))
	for _, m := range supplied {
		suppliedSet[m] = true
	}

	preview := &IntakePreview{Total: len(infos)}
	byIndex := map[int]*IntakePreviewRule{}
	slot := func(i int, action string) *IntakePreviewRule {
		if byIndex[i] == nil {
			byIndex[i] = &IntakePreviewRule{Index: i, Action: action, Examples: []string{}}
		}
		return byIndex[i]
	}
	for i, r := range rules {
		slot(i, r.Action)
	}
	slot(-1, fallbackIntake.Action)

	name = strings.TrimSpace(name)
	for _, info := range infos {
		subject := newIntakeSubject(info, suppliedSet[info.Model])
		i, rule := decideIntakeIndex(rules, subject)
		entry := slot(i, rule.Action)
		entry.Count++
		if list {
			preview.Models = append(preview.Models, IntakePreviewListed{Model: info.Model, Rule: i})
		}
		if len(entry.Examples) < intakePreviewExamples {
			entry.Examples = append(entry.Examples, info.Model)
		}
		if name != "" && strings.EqualFold(info.Model, name) {
			preview.Model = &IntakePreviewModel{Model: info.Model, Known: true, Supplied: subject.supplied, Rule: i,
				Action: rule.Action, JoinLast: rule.joinsLast(subject.supplied)}
		}
	}
	if name != "" && preview.Model == nil {
		subject := intakeSubject{model: name, family: ParseModelFamily(name).Family}
		i, rule := decideIntakeIndex(rules, subject)
		preview.Model = &IntakePreviewModel{Model: name, Rule: i, Action: rule.Action, JoinLast: rule.joinsLast(false)}
	}
	for _, entry := range byIndex {
		preview.Rules = append(preview.Rules, *entry)
	}
	sort.Slice(preview.Rules, func(a, b int) bool {
		ia, ib := preview.Rules[a].Index, preview.Rules[b].Index
		if ia < 0 || ib < 0 {
			return ib < 0 && ia >= 0
		}
		return ia < ib
	})
	return preview, nil
}
