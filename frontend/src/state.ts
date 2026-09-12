import type { Device, Flow } from './model';
export function bindProject(device:Device,projectId:string):Device {
 return device.projectId===projectId ? {...device} : {...device,projectId,linkedRuleSetIds:[],activeRuleSetId:'',lastSelectedRuleSetId:''};
}
export function activate(device:Device,id:string):Device {
 if(id && (!device.projectId || !device.linkedRuleSetIds.includes(id)))throw new Error('请先保存同项目的规则集关联');
 return {...device,activeRuleSetId:id,lastSelectedRuleSetId:id || device.lastSelectedRuleSetId || device.activeRuleSetId};
}
export function filterFlows(flows:Flow[],ip:string,query:string):Flow[] {
 const q=query.trim().toLowerCase();
 return flows.filter(f=>(!ip || f.ip===ip) && (!q || `${f.method} ${f.url} ${f.ip}`.toLowerCase().includes(q)));
}
export function groupFlows(flows:Flow[]) {
 const hosts=new Map<string,{paths:Map<string,Flow[]>;tunnels:Flow[];count:number}>();
 for(const flow of flows) {
  let host='无法解析的地址', path=flow.url;
  try { const url=new URL(flow.url); host=url.host; path=url.pathname; } catch { /* Keep malformed request visible. */ }
  if(!hosts.has(host))hosts.set(host,{paths:new Map(),tunnels:[],count:0});
  const group=hosts.get(host)!;group.count++;
  if(flow.method==='CONNECT'){group.tunnels.push(flow);continue}
  const paths=group.paths;
  if(!paths.has(path))paths.set(path,[]);
  paths.get(path)!.push(flow);
 }
 return [...hosts].map(([host,group])=>({host,count:group.count,tunnels:group.tunnels,paths:[...group.paths].map(([path,flows])=>({path,flows}))}));
}
