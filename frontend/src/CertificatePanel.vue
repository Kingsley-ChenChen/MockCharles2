<script setup lang="ts">
import {onMounted,ref} from 'vue';
import {api} from './api';
import ConnectionGuide from './ConnectionGuide.vue';
import type {CertificateInfo} from './model';
defineProps<{proxyAddress:string;listenAddress:string}>();
const guide=ref<InstanceType<typeof ConnectionGuide>>();
const info=ref<CertificateInfo>(),error=ref(''),notice=ref(''),busy=ref(false);
async function load(){try{info.value=await api().CertificateInfo();error.value=''}catch(e){error.value=String(e)}}
async function generate(){busy.value=true;error.value='';try{info.value=await api().GenerateCertificate();notice.value='本机 CA 已生成。';await guide.value?.open()}catch(e){error.value=String(e)}finally{busy.value=false}}
async function download(){busy.value=true;error.value='';try{const path=await api().ExportCertificate();if(path)notice.value='公共证书已保存：'+path}catch(e){error.value=String(e)}finally{busy.value=false}}
onMounted(load);
</script>
<template>
 <section class="cert-panel"><div class="cert-summary"><h2>本机证书</h2><span class="tag">{{info?.available?'CA 已生成':'尚未生成 CA'}}</span></div>
  <p v-if="error" class="error" role="alert">{{error}} <button @click="load">重新读取证书</button></p><p v-if="notice" role="status">{{notice}}</p>
  <div><div class="actions"><button :disabled="busy || info?.available" @click="generate">生成本机 CA</button><button :disabled="busy || !info?.available" @click="download">保存公共证书</button></div>
   <template v-if="info?.available"><p class="tiny">有效期至 {{new Date(info.notAfter).toLocaleDateString()}}</p><p class="mono wrap tiny">SHA-256：{{info.fingerprint}}</p></template>
   <p><button @click="guide?.open()">手机接入与证书安装</button></p>
  </div>
 </section>
 <ConnectionGuide ref="guide" :listen-address="listenAddress" :proxy-address="proxyAddress" :certificate-available="!!info?.available" />
</template>
