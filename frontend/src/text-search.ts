export function textMatches(text:string,query:string){
 const result:number[]=[];
 if(!query)return result;
 const haystack=text.toLowerCase(),needle=query.toLowerCase();
 for(let from=0;;){const index=haystack.indexOf(needle,from);if(index<0)break;result.push(index);from=index+needle.length}
 return result;
}
