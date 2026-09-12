<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { api } from './api';
import CertificatePanel from './CertificatePanel.vue';
import TrafficTLS from './TrafficTLS.vue';
import { activate, bindProject, filterFlows, groupFlows } from './state';
import type { Config, Device, Flow, Page, Project, Rule, RuleSet, TLSSettings } from './model';

const tlsControls=ref<InstanceType<typeof TrafficTLS>>();
const pages: Record<Page,string> = {devices:'设备管理',rules:'规则管理',sets:'规则集管理',traffic:'流量'};
const config = ref<Config>({revision:0,projects:[],devices:[],rules:[],ruleSets:[]});
const page = ref<Page>('devices'), projectId = ref(''), proxyAddress = ref('');
const listenAddress = ref('0.0.0.0:8888'), error = ref(''), notice = ref(''), busy = ref(false), ready = ref(false);
const loadError = ref('');
const flows = ref<Flow[]>([]), ipFilter = ref(''), search = ref(''), tree = ref(false), selectedFlow = ref<Flow>();
const deviceDraft = ref<Device>(), ruleDraft = ref<Rule>(), setDraft = ref<RuleSet>(), projectDraft = ref<Project>();
const draftRevision = ref(0), candidate = ref(''), headersText = ref('{}'), requestHeadersText = ref('{}'), responseHeadersText = ref('{}');
const projectEditor = ref(false), headerEditor = ref(false), ruleTab = ref('接口规则');
let timer: ReturnType<typeof setInterval> | undefined;
let refreshTask: Promise<void> | undefined;
const project = computed(()=>config.value.projects.find(p=>p.id===projectId.value));
const rules = computed(()=>config.value.rules.filter(r=>r.projectId===projectId.value));
const sets = computed(()=>config.value.ruleSets.filter(s=>s.projectId===projectId.value));
const visibleFlows = computed(()=>filterFlows(flows.value,ipFilter.value,search.value));
const groups = computed(()=>groupFlows(visibleFlows.value));
const linkedSets = computed(()=>config.value.ruleSets.filter(s=>s.projectId===deviceDraft.value?.projectId));
const nameOf = (id:string)=>config.value.ruleSets.find(s=>s.id===id)?.name || '真实转发';
const projectName = (id:string)=>config.value.projects.find(p=>p.id===id)?.name || '未绑定项目';
const uid = ()=>crypto.randomUUID();
const clone = <T,>(value:T):T=>JSON.parse(JSON.stringify(value));
function normalize(c:Config) { c.projects ||= []; c.devices ||= []; c.rules ||= []; c.ruleSets ||= []; c.devices.forEach(d=>d.linkedRuleSetIds ||= []); c.ruleSets.forEach(s=>s.ruleIds ||= []); return c; }
function refresh(): Promise<void> {
 if (refreshTask) return refreshTask;
 if (busy.value) return Promise.resolve();
 refreshTask = loadSnapshot().finally(()=>{refreshTask=undefined});
 return refreshTask;
}
async function loadSnapshot() {
 try {
  const [snapshot, traffic] = await Promise.all([api().Snapshot(), api().Flows()]);
  config.value = normalize(snapshot.config); proxyAddress.value = snapshot.proxyAddress;
  flows.value = traffic || []; if(selectedFlow.value)selectedFlow.value=flows.value.find(f=>f.id===selectedFlow.value!.id); ready.value = true; loadError.value='';
  if (!projectId.value) projectId.value = config.value.projects[0]?.id || '';
 } catch(e) { loadError.value = String(e); }
}
async function action(fn:()=>Promise<void>) {
 if (busy.value) return;
 busy.value = true; error.value = ''; notice.value = '';
 try { await refreshTask; await fn(); notice.value = '操作已完成'; }
 catch(e) { error.value = String(e); }
 finally { busy.value = false; await refresh(); }
}
async function persist(edit:(c:Config)=>void, revision:number) {
 const next = clone(config.value); edit(next); await api().SaveConfig(next,revision);
}
async function saveTLS(settings:TLSSettings,revision:number){
 if(busy.value)throw Error('另一个操作正在执行，请稍后重试');
 busy.value=true;
 try{await refreshTask;await persist(c=>{c.tls=settings},revision)}finally{busy.value=false;await refresh()}
}
function openDevice(d:Device) { deviceDraft.value=clone(d); draftRevision.value=config.value.revision; candidate.value=d.activeRuleSetId || d.lastSelectedRuleSetId || d.linkedRuleSetIds[0] || ''; }
function changeDeviceProject() { if (!deviceDraft.value) return; const current=config.value.devices.find(d=>d.ip===deviceDraft.value!.ip)!; deviceDraft.value={...bindProject(current,deviceDraft.value.projectId),label:deviceDraft.value.label}; candidate.value=''; }
function saveDevice() { if(!deviceDraft.value)return; const d=clone(deviceDraft.value); if(!d.linkedRuleSetIds.includes(d.lastSelectedRuleSetId || ''))d.lastSelectedRuleSetId=''; action(async()=>{await persist(c=>{c.devices=c.devices.map(x=>x.ip===d.ip?d:x)},draftRevision.value); deviceDraft.value=undefined;}); }
function setActive(d:Device,id:string) { action(async()=>{const updated=activate(d,id); await persist(c=>{c.devices=c.devices.map(x=>x.ip===d.ip?updated:x)},config.value.revision); deviceDraft.value=undefined;}); }
function openRule(r?:Rule) { draftRevision.value=config.value.revision; ruleDraft.value=clone(r || {id:uid(),projectId:projectId.value,name:'',method:'GET',url:'http://',status:200,body:'{\n  "code": 0\n}',headers:{'Content-Type':'application/json'},enabled:true}); headersText.value=JSON.stringify(ruleDraft.value.headers,null,2); }
function parseHeaders(text:string):Record<string,string> { const value=JSON.parse(text); if(!value || Array.isArray(value) || typeof value!=='object' || Object.values(value).some(v=>typeof v!=='string'))throw new Error('Header 必须是名称到字符串值的 JSON 对象'); return value; }
function saveRule() { action(async()=>{const r=clone(ruleDraft.value!); r.headers=parseHeaders(headersText.value); await persist(c=>{const i=c.rules.findIndex(x=>x.id===r.id); i<0?c.rules.push(r):c.rules.splice(i,1,r)},draftRevision.value); ruleDraft.value=undefined;}); }
function openSet(s?:RuleSet) { draftRevision.value=config.value.revision; setDraft.value=clone(s || {id:uid(),projectId:projectId.value,name:'',ruleIds:[]}); }
function moveRule(index:number,delta:number) { const ids=setDraft.value!.ruleIds; const [id]=ids.splice(index,1); ids.splice(index+delta,0,id); }
function saveSet() { action(async()=>{const s=clone(setDraft.value!); await persist(c=>{const i=c.ruleSets.findIndex(x=>x.id===s.id); i<0?c.ruleSets.push(s):c.ruleSets.splice(i,1,s)},draftRevision.value); setDraft.value=undefined;}); }
function openProject(headers=false) { draftRevision.value=config.value.revision; headerEditor.value=headers; projectDraft.value=clone(headers?project.value!:{id:uid(),name:'',requestHeaders:{},responseHeaders:{}}); requestHeadersText.value=JSON.stringify(projectDraft.value.requestHeaders || {},null,2); responseHeadersText.value=JSON.stringify(projectDraft.value.responseHeaders || {},null,2); projectEditor.value=true; }
function saveProject() { action(async()=>{const p=clone(projectDraft.value!); p.requestHeaders=parseHeaders(requestHeadersText.value); p.responseHeaders=parseHeaders(responseHeadersText.value); await persist(c=>{const i=c.projects.findIndex(x=>x.id===p.id); i<0?c.projects.push(p):c.projects.splice(i,1,p)},draftRevision.value); projectId.value=p.id; projectEditor.value=false;}); }
function navigate(next:Page) { if(page.value!==next) {deviceDraft.value=undefined;ruleDraft.value=undefined;setDraft.value=undefined;} page.value=next; }
function showTraffic(ip:string) { ipFilter.value=ip; navigate('traffic'); selectedFlow.value=undefined; }
function changeProject() { ruleDraft.value=undefined; setDraft.value=undefined; }
const hasDraft = computed(()=>!!(projectEditor.value || (page.value==='devices' && deviceDraft.value) || (page.value==='rules' && ruleDraft.value) || (page.value==='sets' && setDraft.value)));
const staleDraft = computed(()=>hasDraft.value && draftRevision.value !== config.value.revision);
function reloadDraft() {
 if(projectEditor.value) { openProject(headerEditor.value); }
 else if(page.value==='rules' && ruleDraft.value) { const current=config.value.rules.find(r=>r.id===ruleDraft.value!.id); if(current)openRule(current); else draftRevision.value=config.value.revision; }
 else if(page.value==='sets' && setDraft.value) { const current=config.value.ruleSets.find(s=>s.id===setDraft.value!.id); if(current)openSet(current); else draftRevision.value=config.value.revision; }
 else if(page.value==='devices' && deviceDraft.value) { const current=config.value.devices.find(d=>d.ip===deviceDraft.value!.ip); if(current)openDevice(current); else deviceDraft.value=undefined; }
 error.value='';
}
const sourceName = (source:string)=>({fixed:'固定 Mock',forward:'真实转发',unsupported:'暂不支持',tunnel:'仅隧道转发',tls_error:'TLS 握手失败'}[source] || source);
onMounted(async()=>{await refresh(); timer=setInterval(refresh,1500)});
onUnmounted(()=>clearInterval(timer));
</script>

<template>
 <div class="app">
  <aside class="sidebar"><div class="brand"><b>≈</b> MockCharles</div><p class="section-label">WORKSPACE</p>
   <button v-for="(title,key) in pages" :key="key" class="nav" :class="{active:page===key}" @click="navigate(key)"><span aria-hidden="true">{{ {devices:'▣',rules:'☷',sets:'▤',traffic:'⌁'}[key] }}</span>{{ title }}</button>
   <div class="side-note">本机工作区<br>开发版 · HTTP / HTTPS</div>
  </aside>
  <header class="topbar"><div class="cluster"><span class="muted">查看项目</span><select aria-label="查看项目" v-model="projectId" @change="changeProject"><option value="" disabled>请先创建项目</option><option v-for="p in config.projects" :value="p.id">{{p.name}}</option></select><button @click="openProject()" :disabled="!ready || busy">＋ 项目</button></div>
   <div class="cluster"><span :class="{green:proxyAddress}">● {{proxyAddress ? '监听 '+proxyAddress : '代理未开启'}}</span><button :disabled="busy || !ready" class="primary" @click="action(()=>proxyAddress ? api().StopProxy() : api().StartProxy(listenAddress))">{{proxyAddress?'停止代理':'开启代理'}}</button></div>
  </header>
  <main>
   <div v-if="error || loadError" class="alert" role="alert">{{error || loadError}}<button @click="error='';refresh()">重试 / 收起</button></div><div v-if="notice" class="notice" role="status">{{notice}}<button @click="notice=''">×</button></div>
   <div class="heading"><div><h1>{{pages[page]}}</h1><p>{{ {devices:'以来源 IP 管理设备，在设备上选择并启用规则集。',rules:'维护接口响应与项目通用 Header。',sets:'定义规则优先级，多台设备可独立使用同一规则集。',traffic:'查看本次会话中的实际请求，关闭软件后清空。'}[page] }}</p></div><span class="badge">HTTPS 验收版 · 功能开发中</span></div>
   <div v-if="staleDraft && !projectEditor" class="alert">配置已更新，当前草稿仍保留。<button :disabled="busy" @click="reloadDraft">重新加载当前配置（放弃草稿）</button></div>
   <div v-if="!ready" class="empty"><h2>{{loadError?'无法加载工作区':'正在加载工作区…'}}</h2><p>{{loadError?'请检查桌面后端连接，再点击重试。':'正在读取本机配置。'}}</p></div>
   <template v-if="ready && page==='devices'">
    <details class="certificate-section"><summary>本机证书与安装</summary><CertificatePanel :proxy-address="proxyAddress" :listen-address="listenAddress" /></details><div class="toolbar"><label>监听地址 <input aria-label="监听地址" v-model="listenAddress" :disabled="!!proxyAddress" class="mono"></label><span class="muted">手机代理填写本机局域网 IP 和端口。</span></div>
    <div class="workspace"><section class="list"><div class="list-title">设备来源 <span>{{config.devices.length}} 台</span></div>
     <table><thead><tr><th>来源 IP / 备注</th><th>绑定项目</th><th>启用规则集</th><th></th></tr></thead><tbody><tr v-for="d in config.devices" :key="d.ip" @click="openDevice(d)" :class="{selected:deviceDraft?.ip===d.ip}"><td><strong class="mono">{{d.ip}}</strong><small>{{d.label || '未设置备注'}}</small></td><td>{{projectName(d.projectId)}}</td><td><span class="tag" :class="{green:!!d.activeRuleSetId}">{{nameOf(d.activeRuleSetId)}}</span></td><td><button @click.stop="showTraffic(d.ip)">查看流量</button></td></tr></tbody></table>
     <div v-if="!config.devices.length" class="empty"><h2>等待设备接入</h2><p>开启代理后，让设备通过代理发送一个 HTTP 请求。<br>新 IP 会自动出现，默认真实转发。</p></div>
    </section><aside class="detail" v-if="deviceDraft"><h2>{{deviceDraft.ip}}</h2><p class="muted">保存配置与启用规则集分开操作。</p><label>备注<input v-model="deviceDraft.label"></label><label>绑定项目<select aria-label="绑定项目" v-model="deviceDraft.projectId" @change="changeDeviceProject"><option value="">未绑定项目</option><option v-for="p in config.projects" :value="p.id">{{p.name}}</option></select></label>
     <h3>关联规则集</h3><label class="check" v-for="s in linkedSets"><input type="checkbox" :value="s.id" v-model="deviceDraft.linkedRuleSetIds" :disabled="deviceDraft.activeRuleSetId===s.id">{{s.name}}</label><p v-if="!linkedSets.length" class="muted">此项目还没有规则集。</p>
     <button class="primary" :disabled="busy" @click="saveDevice">保存设备配置</button><hr><template v-if="config.devices.find(d=>d.ip===deviceDraft!.ip)?.linkedRuleSetIds.length"><h3>设备运行</h3><p>当前：{{nameOf(config.devices.find(d=>d.ip===deviceDraft!.ip)!.activeRuleSetId)}}</p><select v-model="candidate" aria-label="待启用规则集"><option value="">选择已保存的关联</option><option v-for="id in config.devices.find(d=>d.ip===deviceDraft!.ip)!.linkedRuleSetIds" :value="id">{{nameOf(id)}}</option></select><div class="actions"><button class="primary" :disabled="busy || !candidate || !proxyAddress" @click="setActive(config.devices.find(d=>d.ip===deviceDraft!.ip)!,candidate)">启用所选</button><button :disabled="busy || !config.devices.find(d=>d.ip===deviceDraft!.ip)!.activeRuleSetId" @click="setActive(config.devices.find(d=>d.ip===deviceDraft!.ip)!,'')">停止设备规则</button></div></template>
    </aside><aside v-else class="detail"><h2>设备与规则集</h2><p>一台设备可关联多个规则集，每次启用一个。设备使用同一规则集时互不影响。</p><hr><h3>HTTPS 证书</h3><p>在上方“本机证书与安装”中生成 CA、下载公共证书并查看安装指引。在流量模块开启 HTTPS 解密并选择需要解密的域名。</p></aside></div>
   </template>
   <template v-if="ready && page==='rules'">
    <div class="toolbar"><div class="seg"><button v-for="tab in ['接口规则','通用 Header']" :class="{selected:ruleTab===tab}" @click="ruleTab=tab">{{tab}}</button></div><button v-if="ruleTab==='接口规则'" class="primary" :disabled="!project || busy" @click="openRule()">＋ 新建接口规则</button></div>
    <div class="workspace" v-if="ruleTab==='接口规则'"><section class="list"><div class="list-title">{{project?.name || '未选择项目'}} <span>{{rules.length}} 条规则</span></div><button v-for="r in rules" class="rule-card" @click="openRule(r)"><div><b>{{r.name}}</b><span class="tag">{{r.enabled?'可匹配':'已停用'}}</span></div><p class="mono">{{r.method}} {{r.url}}</p><small>固定响应 · HTTP {{r.status}}</small></button><div class="empty" v-if="!rules.length">创建一个固定响应，再将它加入规则集。</div></section>
     <aside class="detail" v-if="ruleDraft"><h2>编辑接口规则</h2><label>名称<input v-model="ruleDraft.name"></label><div class="form-row"><label>方法<select v-model="ruleDraft.method"><option v-for="m in ['GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS','*']">{{m}}</option></select></label><label>状态码<input type="number" v-model.number="ruleDraft.status" min="200" max="599"></label></div><label>完整 URL（精确匹配，包含查询参数）<input v-model="ruleDraft.url" class="mono"></label><label>响应 Header（JSON）<textarea v-model="headersText" rows="3" spellcheck="false"></textarea></label><label>固定响应正文<textarea v-model="ruleDraft.body" rows="9" spellcheck="false"></textarea></label><label class="check"><input type="checkbox" v-model="ruleDraft.enabled">允许匹配</label><button class="primary" :disabled="busy" @click="saveRule">保存规则</button></aside><aside v-else class="detail"><h2>固定响应</h2><p>按规则集顺序选择第一条命中的规则。规则保存后对新请求生效。</p><p>随机响应、样本与更多匹配条件将在后续阶段接入。</p></aside>
    </div><section v-else class="panel"><h2>项目通用 Header</h2><p>对绑定此项目的设备统一生效，接口响应 Header 最后覆盖。未绑定设备不改写。</p><div class="columns"><div><h3>请求 Header</h3><pre>{{JSON.stringify(project?.requestHeaders || {},null,2)}}</pre></div><div><h3>响应 Header</h3><pre>{{JSON.stringify(project?.responseHeaders || {},null,2)}}</pre></div></div><button :disabled="!project || busy" @click="openProject(true)">编辑通用 Header</button></section>
   </template>
   <template v-if="ready && page==='sets'">
    <div class="toolbar"><span class="tag">优先匹配</span><span class="muted">顺序执行将在下一阶段接入</span><button class="primary" :disabled="!project || busy" @click="openSet()">＋ 新建规则集</button></div>
    <div class="workspace"><section class="list"><div class="list-title">规则编排 <span>{{sets.length}} 个规则集</span></div><button class="rule-card" v-for="s in sets" @click="openSet(s)"><div><b>{{s.name}}</b><span>{{s.ruleIds.length}} 条规则</span></div><p>关联 {{config.devices.filter(d=>d.linkedRuleSetIds.includes(s.id)).length}} 台设备</p></button><div v-if="!sets.length" class="empty">新建规则集，按优先级添加接口规则。</div></section>
     <aside class="detail" v-if="setDraft"><h2>规则集定义</h2><label>名称<input v-model="setDraft.name"></label><h3>匹配顺序</h3><div v-for="(id,i) in setDraft.ruleIds" class="step"><span>{{i+1}}. {{config.rules.find(r=>r.id===id)?.name}}</span><button :disabled="i===0" @click="moveRule(i,-1)">↑</button><button :disabled="i===setDraft.ruleIds.length-1" @click="moveRule(i,1)">↓</button><button @click="setDraft.ruleIds.splice(i,1)">移除</button></div><select aria-label="添加规则" @change="setDraft.ruleIds.push(($event.target as HTMLSelectElement).value); ($event.target as HTMLSelectElement).value='' "><option value="">＋ 添加接口规则</option><option v-for="r in rules.filter(r=>!setDraft!.ruleIds.includes(r.id))" :value="r.id">{{r.name}}</option></select><button class="primary" :disabled="busy" @click="saveSet">保存编排</button><hr><h3>关联设备 · 只读</h3><div v-for="d in config.devices.filter(d=>d.linkedRuleSetIds.includes(setDraft!.id))" class="association"><span>{{d.ip}}<small>{{d.activeRuleSetId===setDraft.id?'已启用':'未启用'}}</small></span><button @click="page='devices';openDevice(d)">管理设备</button></div></aside><aside v-else class="detail"><h2>运行状态属于设备</h2><p>在这里维护规则定义和顺序，前往设备管理启用。调整优先级对新请求生效。</p></aside></div>
   </template>
   <template v-if="ready && page==='traffic'">
    <TrafficTLS ref="tlsControls" :config="config" :busy="busy" :save="saveTLS" @certificates="navigate('devices')" />
    <div class="toolbar"><select aria-label="流量来源" v-model="ipFilter"><option value="">所有设备流量</option><option v-for="d in config.devices" :value="d.ip">{{d.ip}}</option></select><div class="seg"><button :class="{selected:!tree}" @click="tree=false">流式</button><button :class="{selected:tree}" @click="tree=true">目录式</button></div><input placeholder="搜索 URL / IP / 方法" v-model="search"><span class="muted">{{visibleFlows.length}} 条请求</span><button :disabled="busy || !flows.length" @click="action(async()=>{await api().ClearFlows();selectedFlow=undefined})">清空本次流量</button><button v-if="tree" disabled title="导出格式尚待确认">导出（后续阶段）</button></div>
    <div class="workspace traffic"><section class="list"><table v-if="!tree"><thead><tr><th>方法</th><th>URL</th><th>来源 IP</th><th>状态</th><th>耗时</th></tr></thead><tbody><tr v-for="f in visibleFlows" @click="selectedFlow=f" @contextmenu.prevent="tlsControls?.openFor($event,f)" @keydown.shift.f10.prevent="tlsControls?.openFor($event,f)" tabindex="0" :key="f.id" :class="{selected:selectedFlow?.id===f.id}"><td>{{f.method}}</td><td class="url" :title="f.url">{{f.url}}<small>{{sourceName(f.source)}}</small></td><td class="mono">{{f.ip}}</td><td>{{f.status}}</td><td>{{Math.round(f.duration/1000000)}} ms</td></tr></tbody></table>
     <div v-else class="tree"><details v-for="g in groups" open><summary>{{g.host}}</summary><details v-for="p in g.paths" open><summary>{{p.path}} <span class="muted">{{p.flows.length}}</span></summary><button v-for="f in p.flows" @click="selectedFlow=f" @contextmenu.prevent="tlsControls?.openFor($event,f)" @keydown.shift.f10.prevent="tlsControls?.openFor($event,f)" tabindex="0" :key="f.id">{{f.method}} · {{f.status}} · {{f.ip}} · {{new Date(f.start).toLocaleTimeString()}}</button></details></details></div><div v-if="!visibleFlows.length" class="empty"><h2>暂无流量</h2><p>{{search || ipFilter ? '当前筛选条件下没有请求。' : '开启代理并发送 HTTP 请求后，这里显示真实流量。'}}</p></div>
    </section><aside class="detail flow-detail" v-if="selectedFlow"><h2>{{selectedFlow.method}} · {{selectedFlow.status}}</h2><p class="mono wrap">{{selectedFlow.url}}</p><p>{{selectedFlow.ip}} · {{sourceName(selectedFlow.source)}}</p><p v-if="selectedFlow.error" class="error">{{selectedFlow.error}}</p><button @click="tlsControls?.openFor($event,selectedFlow)">请求操作</button><p v-if="selectedFlow.source==='tunnel'" class="muted">此记录仅为加密隧道，没有明文正文。选择“当前域名解密”并重新连接后，查看新请求。</p><h3>请求 Header</h3><pre>{{JSON.stringify(selectedFlow.requestHeaders,null,2)}}</pre><h3>请求正文</h3><pre>{{selectedFlow.requestBody || '（空）'}}</pre><h3>响应 Header</h3><pre>{{JSON.stringify(selectedFlow.responseHeaders,null,2)}}</pre><h3>响应正文</h3><pre>{{selectedFlow.responseBody || '（空）'}}</pre></aside><aside v-else class="detail"><h2>请求详情</h2><p>选择一条请求查看 Header 与正文。</p><p>当前记录有容量限制，正文仅捕获前段；不影响实际转发内容。</p></aside></div>
   </template>
  </main><footer><span>{{ready?'● 本机工作区':'○ 正在连接'}} · {{flows.length}} 条请求</span><span>配置持久保存 · 流量仅限本次会话</span></footer>
 </div>
 <div v-if="projectEditor" class="overlay"><section role="dialog" aria-modal="true" class="modal"><h2>{{headerEditor?'编辑项目通用 Header':'新建项目'}}</h2><label>项目名称<input v-model="projectDraft!.name"></label><label>请求 Header（JSON）<textarea v-model="requestHeadersText" rows="5"></textarea></label><label>响应 Header（JSON）<textarea v-model="responseHeadersText" rows="5"></textarea></label><p class="error" v-if="error">{{error}}</p><button v-if="staleDraft" :disabled="busy" @click="reloadDraft">重新加载当前配置（放弃草稿）</button><div class="actions"><button :disabled="busy" @click="projectEditor=false">取消</button><button class="primary" :disabled="busy" @click="saveProject">保存项目</button></div></section></div>
</template>
