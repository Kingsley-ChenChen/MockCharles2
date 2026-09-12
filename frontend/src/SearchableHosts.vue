<script setup lang="ts">
import {computed,nextTick,ref,watch} from 'vue';
import {textMatches} from './text-search';
const props=defineProps<{modelValue:string;disabled:boolean}>();
const emit=defineEmits<{'update:modelValue':[value:string]}>();
const query=ref(''),active=ref(0),editor=ref<HTMLTextAreaElement>(),scrollTop=ref(0),scrollLeft=ref(0);
const matches=computed(()=>textMatches(props.modelValue,query.value));
const pieces=computed(()=>{let previous=0;const parts:{text:string;match:number}[]=[];matches.value.forEach((start,index)=>{parts.push({text:props.modelValue.slice(previous,start),match:-1},{text:props.modelValue.slice(start,start+query.value.length),match:index});previous=start+query.value.length});parts.push({text:props.modelValue.slice(previous)+'\n',match:-1});return parts});
function sync(){scrollTop.value=editor.value?.scrollTop||0;scrollLeft.value=editor.value?.scrollLeft||0}
async function reveal(){await nextTick();const at=matches.value[active.value],el=editor.value;if(at===undefined || !el)return;const before=props.modelValue.slice(0,at);el.scrollTop=Math.max(0,(before.split('\n').length-1)*20-40);el.scrollLeft=Math.max(0,(at-before.lastIndexOf('\n')-1)*7.2-40);sync()}
function move(delta:number){if(matches.value.length){active.value=(active.value+delta+matches.value.length)%matches.value.length;reveal()}}
watch([query,()=>props.modelValue],()=>{active.value=0;if(query.value)reveal()});
</script>
<template>
 <div class="host-search"><input v-model="query" type="search" aria-label="搜索解密域名" placeholder="搜索域名（不区分大小写）"><span role="status">{{matches.length}} 处匹配<span v-if="matches.length"> · {{active+1}}/{{matches.length}}</span></span><button :disabled="!matches.length" type="button" aria-label="上一个匹配" @click="move(-1)">↑</button><button :disabled="!matches.length" type="button" aria-label="下一个匹配" @click="move(1)">↓</button></div>
 <div class="hosts-editor"><div class="highlight-window" aria-hidden="true"><pre :style="{transform:`translate(${-scrollLeft}px,${-scrollTop}px)`}"><template v-for="(piece,index) in pieces" :key="index"><mark v-if="piece.match>=0" :class="{current:piece.match===active}">{{piece.text}}</mark><template v-else>{{piece.text}}</template></template></pre></div><textarea ref="editor" aria-label="HTTPS 解密域名" :value="modelValue" :disabled="disabled" wrap="off" spellcheck="false" @input="emit('update:modelValue',($event.target as HTMLTextAreaElement).value)" @scroll="sync"></textarea></div>
</template>
<style scoped>
.host-search{display:flex;gap:6px;align-items:center;margin:10px 0}.host-search input{min-width:0;flex:1}.host-search span{font-size:11px;color:#8493aa;white-space:nowrap}.host-search button{padding:5px 9px}.hosts-editor{position:relative;border:1px solid #dfe5ef;border-radius:6px;overflow:hidden;background:white}.hosts-editor:focus-within{outline:2px solid #4967eb;outline-offset:1px}.highlight-window{position:absolute;inset:0;overflow:hidden;pointer-events:none}.highlight-window pre,.hosts-editor textarea{font:12px/20px Consolas,monospace!important;letter-spacing:normal;padding:12px!important;margin:0!important;border:0!important;border-radius:0;white-space:pre!important;overflow-wrap:normal!important;tab-size:4}.highlight-window pre{color:#25324a;background:transparent;min-width:100%;width:max-content}.hosts-editor textarea{position:relative;display:block;width:100%;height:180px;min-height:120px;resize:vertical;background:transparent!important;color:transparent!important;caret-color:#25324a;outline:none!important;box-shadow:none!important}.hosts-editor textarea:disabled{opacity:.6}.highlight-window mark{color:inherit;background:#ffeb8a;border-radius:2px}.highlight-window mark.current{background:#ffcf53;box-shadow:0 0 0 1px #d39813}
</style>
