import { describe, expect, it } from 'vitest';
import { bindProject, activate, filterFlows, groupFlows } from './state';
import type { Device, Flow } from './model';
describe('device commands', () => {
  const device: Device = {ip:'10.0.0.1',label:'phone',projectId:'p1',linkedRuleSetIds:['s1','s2'],activeRuleSetId:'s1'};
  it('changing a project clears invalid associations without changing source', () => {
    expect(bindProject(device,'p2')).toEqual({...device,projectId:'p2',linkedRuleSetIds:[],activeRuleSetId:'',lastSelectedRuleSetId:''});
    expect(device.activeRuleSetId).toBe('s1');
  });
  it('requires association and only activates one set', () => {
    expect(() => activate(device,'s3')).toThrow();
    expect(activate(device,'s2').activeRuleSetId).toBe('s2');
    expect(activate(device,'').linkedRuleSetIds).toEqual(['s1','s2']);
    expect(activate(activate(device,'s2'),'').lastSelectedRuleSetId).toBe('s2');
  });
});
describe('traffic browsing', () => {
  const flows = [{id:'1',ip:'a',url:'http://a.test/x?q=1',method:'GET'},{id:'2',ip:'b',url:'http://a.test/x?q=2',method:'POST'}] as Flow[];
  it('combines device and search filters', () => {
    expect(filterFlows(flows,'a','GET').map(f=>f.id)).toEqual(['1']);
    expect(filterFlows(flows,'a','POST')).toEqual([]);
  });
  it('directory preserves each request instance', () => {
    expect(groupFlows(flows)[0].paths[0].flows.map(f=>f.id)).toEqual(['1','2']);
  });
});
