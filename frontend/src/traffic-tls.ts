import type {Flow,TLSSettings} from './model';
export function flowHost(flow:Pick<Flow,'url'|'method'>):string {
 try {
  const url=new URL(flow.method==='CONNECT' && !flow.url.includes('://')?'https://'+flow.url:flow.url);
  if(!['http:','https:'].includes(url.protocol) || url.username || url.password)return '';
  return url.hostname.replace(/^\[|\]$/g,'').replace(/\.$/,'').toLowerCase();
 } catch {return ''}
}
export function enableHost(settings:TLSSettings|undefined,host:string):TLSSettings {
 const hosts=[...(settings?.hosts || [])];
 if(!hosts.some(value=>scopeMatches(value,host)))hosts.push(host);
 return {enabled:true,hosts};
}
export function scopeMatches(scope:string,host:string){
 const value=scope.toLowerCase().replace(/\.$/,'');
 if(value.startsWith('*.')){const base=value.slice(2);return host===base || host.endsWith('.'+base)}
 return value===host;
}
