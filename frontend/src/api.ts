import type {Config,Flow,Snapshot} from './model';
export interface API {
 Snapshot(): Promise<Snapshot>;
 SaveConfig(config: Config, revision: number): Promise<void>;
 StartProxy(address: string): Promise<void>;
 StopProxy(): Promise<void>;
 Flows(): Promise<Flow[]>;
 ClearFlows(): Promise<void>;
}
declare global { interface Window { go?: { main: { App: API } } } }
export function api(): API {
 const bridge = window.go?.main.App;
 if (!bridge) throw new Error('尚未连接桌面后端。请运行 MockCharles 桌面程序，或使用 wails dev。');
 return bridge;
}
