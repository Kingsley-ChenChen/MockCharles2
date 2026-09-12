import {expect,it} from 'vitest';
import {flowHost,enableHost} from './traffic-tls';
it('extracts exact hosts from URLs and CONNECT without ports or paths',()=>{
 expect(flowHost({method:'GET',url:'https://API.Example.com:8443/a?q=1'})).toBe('api.example.com');
 expect(flowHost({method:'CONNECT',url:'api.example.com:443'})).toBe('api.example.com');
 expect(flowHost({method:'CONNECT',url:'https://[2001:db8::1]:443'})).toBe('2001:db8::1');
 expect(flowHost({method:'GET',url:'http://api.example.com./'})).toBe('api.example.com');
 for(const url of ['broken','file:///x','https://user:pass@example.com','https://'])expect(flowHost({method:'GET',url})).toBe('');
});
it('enables one exact host without dropping existing hosts or duplicating normalized names',()=>{
 const previous={enabled:false,hosts:['API.EXAMPLE.COM.','other.test']};
 expect(enableHost(previous,'api.example.com')).toEqual({...previous,enabled:true});
 expect(enableHost(previous,'new.test').hosts).toEqual([...previous.hosts,'new.test']);
 expect(previous.enabled).toBe(false);
});
