<script setup lang="ts">
import {computed,nextTick,ref} from 'vue';
import {api} from './api';
import SearchableHosts from './SearchableHosts.vue';
import {enableHost,flowHost,scopeMatches} from './traffic-tls';
import type {Config,Flow,TLSSettings} from './model';
const props=defineProps<{config:Config;busy:boolean;save:(settings:TLSSettings,revision:number)=>Promise<void>}>();
const emit=defineEmits<{certificates:[]}>();
const dialog=ref<HTMLDialogElement>(),menuButton=ref<HTMLButtonElement>();
const menu=ref<{host:string;x:number;y:number}>();
const working=ref(false),error=ref(''),notice=ref(''),needsCA=ref(false);
const shown=ref(false);
const hosts=ref(''),enabled=ref(false),revision=ref(0);
const locked=computed(()=>props.busy || working.value);
const stale=computed(()=>revision.value!==props.config.revision);
const alreadyEnabled=computed(()=>!!(menu.value && props.config.tls?.enabled && props.config.tls.hosts.some(host=>scopeMatches(host,menu.value!.host))));
function reload(){hosts.value=(props.config.tls?.hosts||[]).join('\n');enabled.value=props.config.tls?.enabled||false;revision.value=props.config.revision;error.value='';needsCA.value=false}
async function open(){menu.value=undefined;reload();shown.value=true;await nextTick();dialog.value?.showModal()}
async function openFor(event:MouseEvent|KeyboardEvent,flow:Flow){
 const rect=(event.currentTarget as HTMLElement).getBoundingClientRect();
 const x=event instanceof MouseEvent?event.clientX:rect.left;
 const y=event instanceof MouseEvent?event.clientY:rect.bottom;
 menu.value={host:flowHost(flow),x:Math.max(8,Math.min(x,innerWidth-292)),y:Math.max(8,Math.min(y,innerHeight-150))};
 await nextTick();if(menuButton.value?.disabled)(menuButton.value.nextElementSibling as HTMLButtonElement)?.focus();else menuButton.value?.focus();
}
async function commit(settings:TLSSettings,expected:number):Promise<boolean>{
 if(locked.value)return false;
 working.value=true;error.value='';notice.value='';needsCA.value=false;
 try{
  if(settings.enabled){
   if(!settings.hosts.length)throw Error('请添加至少一个需要解密的域名。');
   let ca;
   try{ca=await api().CertificateInfo()}catch{needsCA.value=true;throw Error('无法读取本机证书，请到设备页检查。')}
   if(!ca.available || Date.parse(ca.notAfter)<=Date.now()){needsCA.value=true;throw Error('请先在设备页生成有效 CA，并在测试设备安装、信任证书。')}
  }
  await props.save(settings,expected);
  notice.value=settings.enabled?'已启用指定域名解密。请重新连接测试 App，让新连接使用新设置；历史流量不会重新解密。':'HTTPS 解密已关闭，新连接仅做隧道转发；已有连接请重新连接。';
  return true;
 }catch(e){error.value=String(e);return false}finally{working.value=false}
}
async function saveDraft(){if(await commit({enabled:enabled.value,hosts:hosts.value.split(/\r?\n/).map(x=>x.trim()).filter(Boolean)},revision.value))dialog.value?.close()}
async function toggle(){
 if(!props.config.tls?.enabled && !props.config.tls?.hosts.length){await open();enabled.value=true;return}
 await commit({enabled:!props.config.tls?.enabled,hosts:[...(props.config.tls?.hosts||[])]},props.config.revision);
}
async function decrypt(){const host=menu.value?.host;menu.value=undefined;if(host)await commit(enableHost(props.config.tls,host),props.config.revision)}
function certificates(){dialog.value?.close();menu.value=undefined;emit('certificates')}
defineExpose({openFor});
</script>
<template>
 <section class="tls-controls" aria-label="HTTPS 解密控制">
  <div class="tls-bar"><strong>HTTPS 解密</strong><span class="tag" :class="{green:config.tls?.enabled}">{{config.tls?.enabled?'已开启 · 指定域名':'未开启'}}</span><span class="muted">{{config.tls?.hosts.length || 0}} 个域名 · 对所有设备生效</span><button :disabled="locked" @click="toggle">{{config.tls?.enabled?'关闭解密':'开启解密'}}</button><button :disabled="locked" @click="open">管理解密域名</button></div>
  <p class="muted">右键请求选择“当前域名解密”。名单外保持加密隧道；测试设备需先安装并信任本机 CA。</p>
  <p v-if="notice" role="status">{{notice}}</p><p v-if="error && !shown" class="error" role="alert">{{error}} <button v-if="needsCA" @click="certificates">前往证书安装</button></p>
 </section>
 <Teleport to="body">
  <div v-if="menu" class="tls-menu-backdrop" @pointerdown.self="menu=undefined" @keydown.esc.stop.prevent="menu=undefined" @keydown.tab="menu=undefined" @contextmenu.prevent="menu=undefined">
   <div role="menu" aria-label="请求操作" class="tls-menu" :style="{left:menu.x+'px',top:menu.y+'px'}"><p class="mono">{{menu.host || '无法识别此请求的域名'}}{{alreadyEnabled?' · 已启用解密':''}}</p><button ref="menuButton" role="menuitem" :disabled="locked || !menu.host || alreadyEnabled" @click="decrypt">当前域名解密</button><button role="menuitem" @click="open">管理解密域名</button></div>
  </div>
  <dialog ref="dialog" class="tls-dialog" aria-labelledby="tls-title" @close="shown=false" @cancel="working && $event.preventDefault()">
   <h2 id="tls-title">管理 HTTPS 解密域名</h2><p>范围对所有设备统一生效，解密后按设备所属项目和启用规则集处理。</p>
   <label class="check"><input type="checkbox" v-model="enabled" :disabled="locked">启用指定域名 HTTPS 解密</label>
   <label>解密范围（每行一个域名、IP 或 *.域名）</label><SearchableHosts v-model="hosts" :disabled="locked" />
   <p class="muted">baidu.com 仅匹配自身；*.baidu.com 匹配自身及所有子域名。不含协议、路径和端口。保存后需重新连接。</p>
   <p v-if="error" role="alert" class="error">{{error}} <button v-if="needsCA" @click="certificates">前往证书安装</button></p>
   <p v-if="stale" class="error">配置已更新，草稿仍保留。<button :disabled="locked" @click="reload">重新加载解密配置（放弃草稿）</button></p>
   <div class="actions"><button :disabled="locked" @click="dialog?.close()">取消</button><button class="primary" :disabled="locked || stale" @click="saveDraft">保存 HTTPS 设置</button></div>
  </dialog>
 </Teleport>
</template>
<style scoped>
.tls-controls{border:1px solid var(--line);background:white;border-radius:8px;padding:12px 16px;margin-bottom:14px}.tls-bar{display:flex;align-items:center;flex-wrap:wrap;gap:10px}.tls-controls p{margin:8px 0 0;font-size:12px}.tls-menu-backdrop{position:fixed;inset:0;z-index:100}.tls-menu{position:absolute;width:276px;border:1px solid var(--line);border-radius:8px;box-shadow:0 8px 32px #25304430;background:white;padding:8px}.tls-menu p{margin:4px 8px 8px;overflow-wrap:anywhere;max-height:40px;overflow:auto;font-size:12px}.tls-menu button{display:block;width:100%;border:0;text-align:left}.tls-menu button:hover{background:#eef3ff}.tls-dialog{width:min(540px,calc(100vw - 48px));max-height:85vh;overflow:auto;border:1px solid var(--line);border-radius:12px;padding:24px;color:inherit}.tls-dialog::backdrop{background:#26385755}.tls-dialog label{display:block;margin:16px 0}.tls-dialog textarea{display:block;width:100%;margin-top:8px}.tls-dialog p{font-size:12px;line-height:1.8}.tls-dialog .check{display:flex;align-items:center;gap:8px}
</style>
