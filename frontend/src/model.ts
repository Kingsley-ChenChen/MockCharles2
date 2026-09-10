export interface Project { id: string; name: string; requestHeaders: Record<string,string>; responseHeaders: Record<string,string> }
export interface Device { ip: string; label: string; projectId: string; linkedRuleSetIds: string[]; activeRuleSetId: string; lastSelectedRuleSetId?: string }
export interface Rule { id: string; projectId: string; name: string; method: string; url: string; status: number; body: string; headers: Record<string,string>; enabled: boolean }
export interface RuleSet { id: string; projectId: string; name: string; ruleIds: string[] }
export interface Config { revision: number; projects: Project[]; devices: Device[]; rules: Rule[]; ruleSets: RuleSet[] }
export interface Snapshot { config: Config; proxyAddress: string }
export interface Flow { id: string; ip: string; method: string; url: string; status: number; source: string; start: string; duration: number; requestHeaders: Record<string,string[]>; responseHeaders: Record<string,string[]>; requestBody: string; responseBody: string; error: string }
export type Page = 'devices' | 'rules' | 'sets' | 'traffic';
