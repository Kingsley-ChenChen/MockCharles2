import {expect,it} from 'vitest';
import {flowHost,enableHost,scopeMatches} from './traffic-tls';
it('extracts exact hosts from URLs and CONNECT without ports or paths',()=>{
 expect(flowHost({method:'GET',url:'https://API.Example.com:8443/a?q=1'})).toBe('api.example.com');
 expect(flowHost({method:'CONNECT',url:'api.example.com:443'})).toBe('api.example.com');
 expect(flowHost({method:'CONNECT',url:'https://[2001:db8::1]:443'})).toBe('2001:db8::1');
 expect(flowHost({method:'GET',url:'http://api.example.com./'})).toBe('api.example.com');
 for(const url of ['broken','file:///x','https://user:pass@example.com','https://'])expect(flowHost({method:'GET',url})).toBe('');
});
it('recognizes parent scopes in the menu without expanding exact entries',()=>{
 expect(scopeMatches('*.BAIDU.COM.','hm.baidu.com')).toBe(true);
 expect(scopeMatches('*.baidu.com','baidu.com')).toBe(true);
 expect(scopeMatches('*.baidu.com','evilbaidu.com')).toBe(false);
 expect(scopeMatches('baidu.com','hm.baidu.com')).toBe(false);
 expect(enableHost({enabled:false,hosts:['*.baidu.com']},'www.baidu.com')).toEqual({enabled:true,hosts:['*.baidu.com']});
});
it('enables one exact host without dropping existing hosts or duplicating normalized names',()=>{
 const previous={enabled:false,hosts:['API.EXAMPLE.COM.','other.test']};
 expect(enableHost(previous,'api.example.com')).toEqual({...previous,enabled:true});
 expect(enableHost(previous,'new.test').hosts).toEqual([...previous.hosts,'new.test']);
 expect(previous.enabled).toBe(false);
});
