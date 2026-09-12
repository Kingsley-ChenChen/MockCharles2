<script setup lang="ts">
import {computed} from 'vue';
import type {Flow} from './model';
const props=defineProps<{flow:Flow;selected:boolean;singleIP:boolean}>();
const emit=defineEmits<{select:[flow:Flow];menu:[event:MouseEvent|KeyboardEvent,flow:Flow]}>();
const tunnel=computed(()=>props.flow.method==='CONNECT');
const target=computed(()=>{try{const u=new URL(props.flow.url);return u.hostname+':'+(u.port || (u.protocol==='https:'?'443':'80'))}catch{return props.flow.url}});
const query=computed(()=>{try{return new URL(props.flow.url).search}catch{return ''}});
const time=computed(()=>new Date(props.flow.start).toLocaleTimeString('zh-CN',{hour12:false}));
</script>
<template>
 <button class="flow-row" :class="{picked:selected,tunnel}" :aria-label="`${flow.method} · ${flow.status} · ${flow.ip} · ${time}`" :title="`${flow.url}\n来源：${flow.ip}\n${flow.error || (tunnel?'仅加密隧道，路径不可见':'')}`" @click="emit('select',flow)" @contextmenu.prevent="emit('menu',$event,flow)" @keydown.shift.f10.prevent="emit('menu',$event,flow)">
  <span class="method" :class="{connect:tunnel,post:flow.method==='POST'}">{{flow.method}}</span>
  <span class="identity"><span class="request-name">{{tunnel?target:(flow.source==='fixed'?'固定 Mock':'请求')}}<span v-if="!tunnel && query" class="query"> {{query}}</span></span><span v-if="!singleIP" class="ip">{{flow.ip}}</span><span v-if="tunnel" class="tunnel-note">路径不可见<span v-if="flow.source==='tls_error'"> · TLS 握手失败</span><span v-else-if="flow.error"> · 连接异常</span></span></span>
  <span class="status" :class="{failed:flow.status>=400 || !!flow.error}">{{flow.status || '—'}}</span><span class="duration">{{Math.round(flow.duration/1000000)}} ms</span><time :title="new Date(flow.start).toLocaleString()">{{time}}</time>
 </button>
</template>
<style scoped>
.flow-row{display:grid;grid-template-columns:65px minmax(80px,1fr) 32px 58px 65px;align-items:center;gap:8px;width:100%;border:0;border-radius:0;padding:7px 12px;text-align:left;min-height:34px;background:transparent;font-size:11px}.flow-row:hover{background:#f5f7fd}.flow-row.picked{background:#edf2ff;box-shadow:inset 3px 0 #4967eb}.method{font:600 10px/1.5 Consolas,monospace;color:#18816d;background:#eaf7f2;border-radius:4px;padding:2px 5px;justify-self:start;min-width:40px;text-align:center}.method.connect{color:#78879f;background:#edf0f5}.method.post{color:#4d66bb;background:#eef1ff}.identity{min-width:0;display:flex;gap:7px;align-items:center;flex-wrap:wrap;color:#61738e}.request-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:100%}.query{color:#8898af;font-family:Consolas,monospace}.ip{font:10px Consolas,monospace;color:#8c9ab0}.tunnel .identity{gap:2px 7px}.tunnel .request-name{font:11px Consolas,monospace}.tunnel-note{width:100%;font-size:10px;color:#95a1b3}.status{color:#21826b;font:11px Consolas,monospace}.status.failed{color:#c84e5c}.duration,time{font:10px Consolas,monospace;color:#8999b0;text-align:right}.flow-row:focus-visible{outline:2px solid #4967eb;outline-offset:-2px}
</style>
