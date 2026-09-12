// Browser-only fixture; production uses the Go bridge.
const {chromium}=require(process.env.PLAYWRIGHT_PATH || 'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true});
 try {
  const page=await browser.newPage({viewport:{width:1100,height:800}}),errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(()=>{
   let config={revision:1,projects:[],devices:[{ip:'10.0.0.2',projectId:'',label:'phone',linkedRuleSetIds:[],activeRuleSetId:''}],rules:[],ruleSets:[],tls:{enabled:false,hosts:[]}};
   const flow={id:'tunnel',ip:'10.0.0.2',method:'CONNECT',url:'https://api.example.test:443',status:200,source:'tunnel',start:new Date().toISOString(),duration:0,requestHeaders:{},responseHeaders:{},requestBody:'',responseBody:'',error:''};
   window.go={main:{App:{Snapshot:async()=>structuredClone({config,proxyAddress:'0.0.0.0:8888'}),Flows:async()=>[flow],CertificateInfo:async()=>({available:false}),SaveConfig:async(c,r)=>{if(r!==config.revision)throw Error('revision conflict');config={...JSON.parse(JSON.stringify(c)),revision:r+1}}}}};
  });
  await page.goto('http://127.0.0.1:4174');await page.getByRole('button',{name:'流量',exact:true}).click();
  const row=page.locator('tbody tr').filter({hasText:'api.example.test'});
  await row.click({button:'right'});await page.getByRole('menuitem',{name:'当前域名解密',exact:true}).click();
  await page.getByRole('alert').filter({hasText:'请先在设备页生成有效 CA'}).waitFor();
  assert.equal((await page.evaluate(()=>window.go.main.App.Snapshot())).config.tls.enabled,false);
  await page.evaluate(()=>{window.go.main.App.CertificateInfo=async()=>({available:true,notAfter:'2031-01-01'})});
  await row.click({button:'right'});await page.getByRole('menuitem',{name:'当前域名解密',exact:true}).click();
  await page.getByRole('button',{name:'关闭解密',exact:true}).waitFor();
  let snap=await page.evaluate(()=>window.go.main.App.Snapshot());assert.deepEqual(snap.config.tls,{enabled:true,hosts:['api.example.test']});assert.equal(snap.config.devices[0].projectId,'');
  await row.click({button:'right'});assert.equal(await page.getByRole('menuitem',{name:'当前域名解密',exact:true}).isDisabled(),true);await page.keyboard.press('Escape');
  await page.evaluate(()=>{window.go.main.App.CertificateInfo=async()=>{throw Error('CA unavailable')}});
  await page.getByRole('button',{name:'关闭解密',exact:true}).click();await page.getByRole('button',{name:'开启解密',exact:true}).waitFor();
  await page.getByRole('button',{name:'管理解密域名',exact:true}).click();
  const dialog=page.getByRole('dialog');await dialog.getByLabel('HTTPS 解密域名').fill('draft.test');
  await page.evaluate(async()=>{const a=window.go.main.App,{config}=await a.Snapshot();await a.SaveConfig(config,config.revision)});
  await dialog.getByRole('button',{name:'重新加载解密配置（放弃草稿）'}).waitFor();
  assert.equal(await dialog.getByLabel('HTTPS 解密域名').inputValue(),'draft.test');assert.equal(await dialog.getByRole('button',{name:'保存 HTTPS 设置'}).isDisabled(),true);
  await dialog.getByRole('button',{name:'重新加载解密配置（放弃草稿）'}).click();assert.equal(await dialog.getByLabel('HTTPS 解密域名').inputValue(),'api.example.test');
  await page.evaluate(()=>{window.originalSave=window.go.main.App.SaveConfig;window.go.main.App.SaveConfig=async()=>{throw Error('disk unavailable')}});
  await dialog.getByLabel('HTTPS 解密域名').fill('retry.test');await dialog.getByRole('button',{name:'保存 HTTPS 设置'}).click();await dialog.getByRole('alert').filter({hasText:'disk unavailable'}).waitFor();assert.equal(await dialog.getByLabel('HTTPS 解密域名').inputValue(),'retry.test');
  await page.evaluate(()=>{window.go.main.App.SaveConfig=window.originalSave;window.go.main.App.CertificateInfo=async()=>({available:true,notAfter:'2031-01-01'})});
  await dialog.getByRole('button',{name:'保存 HTTPS 设置'}).click();await dialog.waitFor({state:'hidden'});
  await page.getByRole('button',{name:'目录式',exact:true}).click();await page.getByLabel('流量来源').selectOption('10.0.0.2');
  await page.locator('.tree button').first().click({button:'right'});await page.getByRole('menuitem',{name:'当前域名解密',exact:true}).click();await page.getByRole('button',{name:'关闭解密',exact:true}).waitFor();
  snap=await page.evaluate(()=>window.go.main.App.Snapshot());assert.deepEqual(snap.config.tls.hosts,['retry.test','api.example.test']);
  await page.locator('.tree button').first().click();await page.getByText('此记录仅为加密隧道，没有明文正文。选择“当前域名解密”并重新连接后，查看新请求。',{exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  await page.screenshot({path:'../.cache/traffic-https.png',fullPage:true});assert.deepEqual(errors,[]);
  console.log('PASS: TLS menu in stream/tree and IP filter, missing CA, duplicate, disable with unavailable CA, conflict/draft/retry, no device rebinding, 1100px layout');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
