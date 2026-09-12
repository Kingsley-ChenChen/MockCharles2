import {computed,onUnmounted,ref,watch} from 'vue';
export function useTrafficResize(){
 const workspace=ref<HTMLElement>(),available=ref(900),preferred=ref<number>(),dragging=ref(false);
 const minimum=computed(()=>Math.min(260,available.value/2));
 const maximum=computed(()=>Math.max(minimum.value,available.value-348));
 const width=computed(()=>Math.max(minimum.value,Math.min(maximum.value,preferred.value ?? (available.value<960?315:365))));
 const style=computed(()=>({gridTemplateColumns:`minmax(0,1fr) 8px ${width.value}px`}));
 let observer:ResizeObserver|undefined,startX=0,startWidth=0,pointer:number|undefined;
 watch(workspace,element=>{observer?.disconnect();dragging.value=false;pointer=undefined;if(element){available.value=element.clientWidth;observer=new ResizeObserver(()=>{available.value=element.clientWidth});observer.observe(element)}});
 onUnmounted(()=>observer?.disconnect());
 function begin(event:PointerEvent){if(event.button!==0 || pointer!==undefined)return;event.preventDefault();pointer=event.pointerId;startX=event.clientX;startWidth=width.value;dragging.value=true;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)}
 function move(event:PointerEvent){if(event.pointerId!==pointer)return;preferred.value=Math.max(minimum.value,Math.min(maximum.value,startWidth+startX-event.clientX))}
 function end(event:PointerEvent){if(event.pointerId!==pointer)return;pointer=undefined;dragging.value=false;const target=event.currentTarget as HTMLElement;if(target.hasPointerCapture(event.pointerId))target.releasePointerCapture(event.pointerId)}
 function reset(){preferred.value=undefined}
 function key(event:KeyboardEvent){if(event.key==='ArrowLeft'||event.key==='ArrowRight'){event.preventDefault();preferred.value=Math.max(minimum.value,Math.min(maximum.value,width.value+(event.key==='ArrowLeft'?24:-24)))}else if(event.key==='Home'){event.preventDefault();reset()}}
 return {workspace,width,minimum,maximum,style,dragging,begin,move,end,reset,key};
}
