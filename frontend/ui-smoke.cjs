// Explicit browser-only test bridge; never imported by the application.
const {chromium}=require(process.env.PLAYWRIGHT_PATH || 'playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true});
 try {
  const page=await browser.newPage({viewport:{width:1380,height:900}});
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.addInitScript(()=>{
   let config={revision:1,projects:[{id:'p1',name:'商城 App',requestHeaders:{},responseHeaders:{}},{id:'p2',name:'会员中心',requestHeaders:{},responseHeaders:{}}],devices:[{ip:'192.168.1.108',label:'测试手机',projectId:'p1',linkedRuleSetIds:['s1','s2'],activeRuleSetId:'s1'},{ip:'192.168.1.111',label:'另一台设备',projectId:'p1',linkedRuleSetIds:['s1'],activeRuleSetId:'s1'}],rules:[{id:'r1',projectId:'p1',name:'商品列表',method:'GET',url:'http://api.test/items',status:200,body:'{"code":0}',headers:{},enabled:true}],ruleSets:[{id:'s1',projectId:'p1',name:'默认规则集',ruleIds:['r1']},{id:'s2',projectId:'p1',name:'备用规则集',ruleIds:[]}]};
   let certAvailable=false; let address='127.0.0.1:8888';
   let flows=[{id:'1',ip:'192.168.1.108',method:'GET',url:'http://api.test/items',status:200,source:'fixed',start:new Date().toISOString(),duration:1000000,requestHeaders:{},responseHeaders:{},requestBody:'',responseBody:'{"code":0}',error:''}];
   window.go={main:{App:{ConnectionInfo:async()=>({addresses:[{ip:'192.168.1.20',interfaceName:'Wi-Fi'},{ip:'10.8.0.2',interfaceName:'VPN'}],port:'8888',listening:true}),CertificateInfo:async()=>({available:certAvailable,fingerprint:'TEST-FINGERPRINT',notAfter:'2031-01-01'}),GenerateCertificate:async()=>{certAvailable=true;return {available:true,fingerprint:'TEST-FINGERPRINT',notAfter:'2031-01-01'}},ExportCertificate:async()=>'',Snapshot:async()=>JSON.parse(JSON.stringify({config,proxyAddress:address})),SaveConfig:async(c,r)=>{if(r!==config.revision)throw Error('revision conflict'); config={...JSON.parse(JSON.stringify(c)),revision:r+1};},StartProxy:async(a)=>{address=a},StopProxy:async()=>{address=''},Flows:async()=>JSON.parse(JSON.stringify(flows)),ClearFlows:async()=>{flows=[]}}}};
  });
  await page.goto('http://127.0.0.1:4174');await page.waitForLoadState('networkidle');
  await page.getByText('测试手机',{exact:true}).click();
  await page.getByLabel('待启用规则集').selectOption('s2');
  let snap=await page.evaluate(()=>window.go.main.App.Snapshot());assert.equal(snap.config.devices[0].activeRuleSetId,'s1');
  await page.getByRole('button',{name:'启用所选',exact:true}).click();
  await page.waitForFunction(async()=> (await window.go.main.App.Snapshot()).config.devices[0].activeRuleSetId==='s2');
  snap=await page.evaluate(()=>window.go.main.App.Snapshot());assert.equal(snap.config.devices[1].activeRuleSetId,'s1');
  await page.getByLabel('查看项目').selectOption('p2');snap=await page.evaluate(()=>window.go.main.App.Snapshot());assert.equal(snap.config.devices[0].projectId,'p1');
  await page.getByLabel('查看项目').selectOption('p1');
  await page.getByRole('button',{name:'规则集管理'}).click();await page.getByRole('button',{name:/默认规则集/}).click();
  assert.equal(await page.getByRole('button',{name:'启用所选',exact:true}).count(),0);
  await page.getByRole('button',{name:'流量',exact:true}).click();
  await page.getByRole('button',{name:'目录式',exact:true}).click();await page.getByText('api.test',{exact:true}).waitFor();
  await page.getByLabel('流量来源').selectOption('192.168.1.111');await page.getByText('暂无流量',{exact:true}).waitFor();
  await page.getByLabel('流量来源').selectOption('');await page.getByRole('button',{name:/GET · 200/}).click();await page.getByRole('heading',{name:'响应正文'}).waitFor();
  await page.screenshot({path:'../.cache/traffic-app.png',fullPage:true});
  await page.getByRole('button',{name:'设备管理'}).click();await page.getByText('测试手机',{exact:true}).click();
  await page.screenshot({path:'../.cache/devices-app.png',fullPage:true});
  await page.setViewportSize({width:1100,height:760});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  assert.deepEqual(errors,[]); assert.equal(await page.getByRole('alert').count(),0);
  await page.getByLabel('备注',{exact:true}).fill('未保存草稿');
  await page.evaluate(async()=>{const s=await window.go.main.App.Snapshot();s.config.projects[0].name='外部更新';await window.go.main.App.SaveConfig(s.config,s.config.revision)});
  await page.getByRole('button',{name:'保存设备配置',exact:true}).click();
  await page.getByText('Error: revision conflict',{exact:false}).waitFor();
  assert.equal(await page.getByLabel('备注',{exact:true}).inputValue(),'未保存草稿');
  await page.getByRole('button',{name:'重新加载当前配置（放弃草稿）'}).click();
  await page.waitForFunction(()=>document.querySelector('.detail label input')?.value==='测试手机');
  await page.getByLabel('备注',{exact:true}).fill('可继续保存');
  await page.getByRole('button',{name:'保存设备配置',exact:true}).click();
  await page.getByText('可继续保存',{exact:true}).waitFor();
  await page.getByText('本机证书与安装',{exact:true}).click();
  await page.getByRole('button',{name:'生成本机 CA',exact:true}).click(); await page.getByRole('dialog').waitFor(); await page.getByText('192.168.1.20',{exact:true}).waitFor(); assert.equal(await page.getByRole('dialog').getByText('8888',{exact:true}).count(),2); await page.screenshot({path:'../.cache/connection-guide.png',fullPage:true}); await page.keyboard.press('Escape'); await page.getByRole('button',{name:'手机接入与证书安装',exact:true}).click(); await page.getByRole('dialog').getByText('http://mc.invalid',{exact:true}).waitFor(); await page.getByRole('button',{name:'关闭安装指引'}).click();
  assert.equal(await page.getByRole('checkbox',{name:'启用指定域名 HTTPS 解密'}).count(),0); assert.equal(await page.getByLabel('HTTPS 解密域名').count(),0); assert.equal(await page.getByRole('button',{name:'保存 HTTPS 设置'}).count(),0);
  const unavailable=await browser.newPage();await unavailable.goto('http://127.0.0.1:4174');
  await unavailable.getByRole('heading',{name:'无法加载工作区'}).waitFor();
  assert.equal(await unavailable.getByRole('heading',{name:'等待设备接入'}).count(),0);
  await unavailable.close();
  console.log('PASS: device activation/isolation, project browsing, device-only controls, flow filtering/tree/details, 1100px layout, no JS errors');
  console.log('PASS: conflict keeps draft, explicit reload recovers saves, unavailable backend distinguished from empty workspace');
 } finally {await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
