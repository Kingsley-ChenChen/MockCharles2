<script setup lang="ts">
import {onMounted,ref,watch} from 'vue';
import {api} from './api';
import type {CertificateInfo,Config,TLSSettings} from './model';
const props=defineProps<{config:Config;proxyAddress:string;saveSettings:(settings:TLSSettings,revision:number)=>Promise<void>}>();
const emit=defineEmits<{saved:[]}>();
const info=ref<CertificateInfo>(),error=ref(''),notice=ref(''),busy=ref(false),hosts=ref(''),enabled=ref(false),dirty=ref(false),revision=ref(0);
function reload(){hosts.value=(props.config.tls?.hosts || []).join('\n');enabled.value=props.config.tls?.enabled || false;revision.value=props.config.revision;dirty.value=false;error.value='';}
watch(()=>props.config,()=>{if(!dirty.value)reload()},{immediate:true});
async function load(){try{info.value=await api().CertificateInfo();error.value=''}catch(e){error.value=String(e)}}
async function generate(){busy.value=true;error.value='';try{info.value=await api().GenerateCertificate();notice.value='本机 CA 已生成，请在测试设备上安装并信任公共证书。'}catch(e){error.value=String(e)}finally{busy.value=false}}
async function download(){busy.value=true;error.value='';try{const path=await api().ExportCertificate();if(path)notice.value='公共证书已保存：'+path}catch(e){error.value=String(e)}finally{busy.value=false}}
async function save(){busy.value=true;error.value='';try{await props.saveSettings({enabled:enabled.value,hosts:hosts.value.split(/\r?\n/).map(x=>x.trim()).filter(Boolean)},revision.value);dirty.value=false;notice.value='HTTPS 设置已保存，新建连接使用新设置；已有连接请重新连接。';emit('saved')}catch(e){error.value=String(e)}finally{busy.value=false}}
onMounted(load);
</script>
<template>
 <section class="cert-panel"><div class="cert-summary"><h2>HTTPS 与本机证书</h2><span class="tag">{{info?.available?'CA 已生成':'尚未生成 CA'}}</span><span class="muted">仅对指定域名解密 · 其他域名只转发隧道</span></div>
  <p v-if="error" class="error" role="alert">{{error}} <button @click="load">重新读取证书</button></p><p v-if="notice" role="status">{{notice}}</p>
  <div class="columns"><div><div class="actions"><button :disabled="busy || info?.available" @click="generate">生成本机 CA</button><button :disabled="busy || !info?.available" @click="download">保存公共证书</button></div>
   <template v-if="info?.available"><p class="tiny">有效期至 {{new Date(info.notAfter).toLocaleDateString()}}</p><p class="mono wrap tiny">SHA-256：{{info.fingerprint}}</p></template>
   <p>手机设置本机代理并开启监听后，在浏览器访问<br><code>http://mockcharles.invalid/ca.crt</code> 下载公共证书。</p>
   <details><summary>iOS / Android 安装与信任说明</summary><p>iOS：下载后在系统设置中安装描述文件，再到“通用 → 关于本机 → 证书信任设置”开启该根证书的完全信任。</p><p>Android：安装用户 CA 后，自研 App 的调试构建还需通过 Network Security Configuration 允许测试 CA。证书固定或绕过系统代理的连接可能仍无法解密。</p><p>证书下载成功不表示设备已信任。私钥只保存在本机；本版不提供自动替换或重建 CA。</p></details>
  </div><div><label class="check"><input type="checkbox" v-model="enabled" @change="dirty=true" :disabled="busy || (!info?.available && !enabled)">启用指定域名 HTTPS 解密</label><label class="host-label">解密域名（每行一个，不含协议、端口或通配符）<textarea aria-label="HTTPS 解密域名" v-model="hosts" @input="dirty=true" rows="3" placeholder="api.example.com&#10;login.example.com"></textarea></label><button class="primary" :disabled="busy || !dirty" @click="save">保存 HTTPS 设置</button><button v-if="dirty && revision!==config.revision" :disabled="busy" @click="reload">加载最新设置（放弃草稿）</button><p class="muted">对所有来源 IP 使用同一解密范围。解密后仍按设备所属项目与启用规则集处理；未绑定设备不改写 Header。</p></div></div>
 </section>
</template>
