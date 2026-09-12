<script setup lang="ts">
import {nextTick,ref,watch} from 'vue';
import {api} from './api';
import type {ConnectionInfo} from './model';
const props=defineProps<{listenAddress:string;proxyAddress:string;certificateAvailable:boolean}>();
const dialog=ref<HTMLDialogElement>(),info=ref<ConnectionInfo>(),loading=ref(false),error=ref('');
let request=0;
async function refresh(){const current=++request;loading.value=true;error.value='';try{const result=await api().PrepareConnection(props.listenAddress);if(current===request)info.value=result}catch(e){if(current===request){info.value=undefined;error.value=String(e)}}finally{if(current===request)loading.value=false}}
async function open(){await nextTick();dialog.value?.showModal();await refresh()}
watch(()=>[props.listenAddress,props.proxyAddress],()=>{if(dialog.value?.open)refresh()});
defineExpose({open});
</script>
<template>
 <Teleport to="body"><dialog ref="dialog" class="connection-guide" aria-labelledby="connection-guide-title">
  <header><div><h2 id="connection-guide-title">手机接入与证书安装</h2><p class="muted">手机与电脑需要处于可互通的局域网。</p></div><button aria-label="关闭安装指引" @click="dialog?.close()">关闭</button></header>
  <h3>1. 在手机 Wi-Fi 设置中配置手动代理</h3>
  <p v-if="loading" role="status">正在准备证书下载服务…</p>
  <p v-else-if="error" role="alert" class="error">{{error}} <button @click="refresh">重新读取</button></p>
  <template v-else-if="info">
   <p v-if="!info.listening" class="guide-warning">证书下载服务尚未就绪，请重试。</p>
   <table v-if="info.addresses.length"><thead><tr><th>本机内网 IP（代理服务器）</th><th>端口</th><th>网卡</th></tr></thead><tbody><tr v-for="address in info.addresses" :key="address.ip"><td class="mono">{{address.ip}}</td><td class="mono">{{info.port==='0'?'开启后自动分配':info.port}}</td><td>{{address.interfaceName}}</td></tr></tbody></table>
   <p v-else class="guide-warning">当前监听地址下没有可供手机连接的内网 IP。请连接局域网，并将监听地址设为 <code>0.0.0.0:8888</code> 或对应内网 IP；不要使用 127.0.0.1。</p>
   <p class="muted tiny">有多个地址时，选择与手机同一网络的 Wi-Fi / 以太网地址；VPN 或虚拟网卡不一定能从手机访问。</p>
   <button @click="refresh">刷新网络地址</button>
  </template>
  <h3>2. 下载公共证书</h3>
  <p v-if="!certificateAvailable" class="guide-warning">请先在设备页生成本机 CA。</p>
  <p>设置好手机代理后，浏览器输入：</p><div class="download-address"><code>http://mc.invalid</code></div>
  <p class="muted tiny">保留 http://。打开本指引会自动准备下载服务，无需开启抓包。配置阶段只提供证书下载，其他网站需开始抓包后访问。</p>
  <h3>3. 安装并信任证书</h3>
  <p><strong>iOS：</strong>在系统设置中安装下载的描述文件，再到“通用 → 关于本机 → 证书信任设置”开启该根证书的完全信任。</p>
  <p><strong>Android：</strong>安装用户 CA 后，自研 App 的调试构建还需通过 Network Security Configuration 允许测试 CA。证书固定或绕过系统代理的连接可能仍无法解密。</p>
  <p class="muted tiny">下载成功不表示设备已信任。私钥只保存在本机，下载内容仅包含公共证书。域名解密的操作入口将在流量模块提供。</p>
 </dialog></Teleport>
</template>
<style scoped>
.connection-guide{width:min(680px,calc(100vw - 48px));max-height:85vh;overflow:auto;border:1px solid var(--line);border-radius:12px;padding:24px;color:inherit;box-shadow:0 18px 75px #1c305a30}.connection-guide::backdrop{background:#26385755}.connection-guide header{display:flex;justify-content:space-between;align-items:flex-start;gap:20px;border-bottom:1px solid var(--line);padding-bottom:10px}.connection-guide h2{font-size:18px}.connection-guide h3{margin-top:22px}.connection-guide p{font-size:12px;line-height:1.8}.connection-guide table{border:1px solid var(--line)}.connection-guide tbody tr{cursor:text}.guide-warning{color:#946214;background:#fff8e9;padding:10px;border-radius:6px}.download-address{background:#eef3ff;border:1px solid #dce5ff;border-radius:8px;padding:14px 18px;color:#3158c8;font-size:20px;user-select:all}.connection-guide .tiny{font-size:11px}
</style>
