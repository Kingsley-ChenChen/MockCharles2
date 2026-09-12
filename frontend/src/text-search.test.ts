import {expect,it} from 'vitest';
import {textMatches} from './text-search';
it('finds literal non-overlapping matches across lines without regex interpretation',()=>{
 expect(textMatches('baidu.com\nWWW.BAIDU.COM\nhm.baidu.com','baidu')).toEqual([0,14,27]);
 expect(textMatches('*.a.test\naXtest','a.test')).toEqual([2]);
 expect(textMatches('anything','')).toEqual([]);
 expect(textMatches('aaa','aa')).toEqual([0]);
});
