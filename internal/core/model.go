package core

import "time"

type Config struct {
	Revision int64     `json:"revision"`
	Projects []Project `json:"projects"`
	Devices  []Device  `json:"devices"`
	Rules    []Rule    `json:"rules"`
	RuleSets []RuleSet `json:"ruleSets"`
}

type Project struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	RequestHeaders  map[string]string `json:"requestHeaders"`
	ResponseHeaders map[string]string `json:"responseHeaders"`
}
type Device struct {
	IP                    string   `json:"ip"`
	Label                 string   `json:"label"`
	ProjectID             string   `json:"projectId"`
	LinkedRuleSetIDs      []string `json:"linkedRuleSetIds"`
	ActiveRuleSetID       string   `json:"activeRuleSetId"`
	LastSelectedRuleSetID string   `json:"lastSelectedRuleSetId"`
}
type Rule struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"projectId"`
	Name      string            `json:"name"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Status    int               `json:"status"`
	Body      string            `json:"body"`
	Headers   map[string]string `json:"headers"`
	Enabled   bool              `json:"enabled"`
}
type RuleSet struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	Name      string   `json:"name"`
	RuleIDs   []string `json:"ruleIds"`
}
type Snapshot struct {
	Config       Config `json:"config"`
	ProxyAddress string `json:"proxyAddress"`
}
type Flow struct {
	ID              string              `json:"id"`
	IP              string              `json:"ip"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	Status          int                 `json:"status"`
	Source          string              `json:"source"`
	Start           time.Time           `json:"start"`
	Duration        time.Duration       `json:"duration"`
	RequestHeaders  map[string][]string `json:"requestHeaders"`
	ResponseHeaders map[string][]string `json:"responseHeaders"`
	RequestBody     string              `json:"requestBody"`
	ResponseBody    string              `json:"responseBody"`
	Error           string              `json:"error"`
}
