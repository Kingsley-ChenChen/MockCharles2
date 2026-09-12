<script setup lang="ts">
import {computed,ref} from 'vue';
import {groupFlows} from './state';
import FlowTreeRow from './FlowTreeRow.vue';
import type {Flow} from './model';
const props=defineProps<{flows:Flow[];selectedId?:string;singleIP:boolean}>();
const emit=defineEmits<{select:[flow:Flow];menu:[event:MouseEvent|KeyboardEvent,flow:Flow]}>();
const groups=computed(()=>groupFlows(props.flows));
const expanded=ref<Record<string,boolean>>({}),defaultOpen=ref(true),tunnelsOpen=ref(false);
const key=(host:string,path?:string)=>JSON.stringify([host,path]);
const isOpen=(id:string,tunnel=false)=>expanded.value[id] ?? (tunnel?tunnelsOpen.value:defaultOpen.value);
function toggle(event:Event,id:string){expanded.value[id]=(event.currentTarget as HTMLDetailsElement).open}
function expandAll(open:boolean){expanded.value={};defaultOpen.value=open;tunnelsOpen.value=open}
defineExpose({expandAll});
</script>
<template>
 <div class="compact-tree" aria-label="流量目录">
  <div class="column-hints"><span>域名 / 路径 / 请求</span><span>状态</span><span>耗时</span><span>时间</span></div>
  <details v-for="g in groups" :key="g.host" class="host" :open="isOpen(key(g.host))" @toggle="toggle($event,key(g.host))">
   <summary><span class="arrow">▶</span><span class="host-icon">◎</span><strong>{{g.host}}</strong><span class="count">{{g.count}}</span></summary>
   <div v-if="g.paths.length" class="paths">
    <details v-for="p in g.paths" :key="p.path" class="path" :open="isOpen(key(g.host,p.path))" @toggle="toggle($event,key(g.host,p.path))">
     <summary><span class="arrow">▶</span><span class="folder">▱</span><span class="path-name" :title="p.path">{{p.path}}</span><span class="count">{{p.flows.length}}</span></summary>
     <div class="requests"><FlowTreeRow v-for="f in p.flows" :key="f.id" :flow="f" :selected="selectedId===f.id" :single-i-p="singleIP" @select="emit('select',$event)" @menu="(event,flow)=>emit('menu',event,flow)" /></div>
    </details>
   </div>
   <details v-if="g.tunnels.length" class="tunnels" :open="isOpen(key(g.host,'@tunnels'),true)" @toggle="toggle($event,key(g.host,'@tunnels'))">
    <summary><span class="arrow">▶</span><span>加密隧道</span><span class="count">{{g.tunnels.length}}</span></summary>
    <FlowTreeRow v-for="f in g.tunnels" :key="f.id" :flow="f" :selected="selectedId===f.id" :single-i-p="singleIP" @select="emit('select',$event)" @menu="(event,flow)=>emit('menu',event,flow)" />
   </details>
  </details>
 </div>
</template>
<style scoped>
.compact-tree{padding:0 0 10px}.column-hints{display:grid;grid-template-columns:1fr 32px 58px 65px;gap:8px;padding:10px 12px 10px 17px;background:#fafbfe;border-bottom:1px solid #edf0f6;color:#97a4b7;font-size:10px}.column-hints span:nth-child(n+3){text-align:right}summary{list-style:none;display:flex;align-items:center;gap:8px;cursor:pointer;min-height:32px}summary::-webkit-details-marker{display:none}summary:hover{background:#f7f9fd}summary:focus-visible{outline:2px solid #4967eb;outline-offset:-2px}.host>summary{padding:9px 16px}.host>summary strong{font-size:12px;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.host>summary>.count{margin-left:auto}.arrow{color:#9cacc4;font-size:9px;flex-shrink:0}.host-icon{color:#8095b8;font-size:15px}details[open]>summary>.arrow{transform:rotate(90deg)}.count{border-radius:10px;min-width:19px;padding:0 6px;font:10px/1.8 Consolas,monospace;color:#8a9bb5;background:#f0f3f9;flex-shrink:0}.paths{margin-left:29px;border-left:1px solid #e7edf6}.path>summary{padding:6px 12px;color:#657793;font-size:11px}.folder{font-size:15px;color:#9aacca}.path-name{font-family:Consolas,monospace;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.requests{padding-left:12px}.tunnels{margin:2px 0 6px 41px}.tunnels>summary{font-size:11px;color:#8a9bb2;padding:5px 0}
</style>
