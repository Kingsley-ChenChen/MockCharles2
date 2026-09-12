// Real Wails + Go + TLS test. Start wails dev with a fresh MOCKCHARLES_DATA_DIR.
const {chromium}=require(process.env.PLAYWRIGHT_PATH || 'playwright');
const http=require('node:http');
const tls=require('node:tls');
const {X509Certificate}=require('node:crypto');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true});
 try{
  const page=await browser.newPage({viewport:{width:1380,height:960}});
  await page.goto('http://127.0.0.1:34115');await page.waitForFunction(()=>!!window.go?.main?.App);
  let snapshot=await page.evaluate(()=>window.go.main.App.Snapshot());assert.equal(snapshot.proxyAddress,'');assert.equal((snapshot.config.projects||[]).length,0,'Use a fresh isolated data directory');
  await page.getByText('本机证书与安装',{exact:true}).click();
  if(!(await page.evaluate(()=>window.go.main.App.CertificateInfo())).available)await page.getByRole('button',{name:'生成本机 CA',exact:true}).click();await page.getByText('CA 已生成',{exact:true}).waitFor();
  if(await page.getByRole('dialog').isVisible())await page.getByRole('button',{name:'关闭安装指引'}).click(); const info=await page.evaluate(()=>window.go.main.App.CertificateInfo());assert.equal(info.available,true);
  assert.equal((await page.evaluate(()=>window.go.main.App.GenerateCertificate())).fingerprint,info.fingerprint);
  // TLS policy is a backend test fixture while its UI entry awaits the traffic-module implementation.
  await page.evaluate(async()=>{const a=window.go.main.App;const {config:c}=await a.Snapshot();c.tls={enabled:true,hosts:['api.example.test']};await a.SaveConfig(c,c.revision)});
  await page.evaluate(async()=>{const a=window.go.main.App;const {config:c}=await a.Snapshot();c.projects=[{id:'p',name:'HTTPS 验收',requestHeaders:{},responseHeaders:{'X-Project':'yes'}}];c.rules=[{id:'r',projectId:'p',name:'安全接口',method:'GET',url:'https://api.example.test/secure',status:200,body:'{"source":"https-mock"}',headers:{'Content-Type':'application/json'},enabled:true}];c.ruleSets=[{id:'s',projectId:'p',name:'HTTPS 规则集',ruleIds:['r']}];c.devices=[{ip:'127.0.0.1',label:'TLS 实际请求',projectId:'p',linkedRuleSetIds:['s'],activeRuleSetId:'s',lastSelectedRuleSetId:'s'}];await a.SaveConfig(c,c.revision)});
  await page.getByLabel('监听地址').fill('127.0.0.1:0');await page.getByRole('button',{name:'开启代理',exact:true}).click();await page.waitForFunction(async()=>!!(await window.go.main.App.Snapshot()).proxyAddress);
  const address=(await page.evaluate(()=>window.go.main.App.Snapshot())).proxyAddress;const port=Number(address.split(':').pop());
  const der=await new Promise((resolve,reject)=>{http.get({host:'127.0.0.1',port,path:'http://mc.invalid/'},res=>{assert.equal(res.statusCode,200);const chunks=[];res.on('data',c=>chunks.push(c));res.on('end',()=>resolve(Buffer.concat(chunks)))}).on('error',reject)});
  const certificate=new X509Certificate(der);assert.equal(certificate.ca,true);assert.equal(certificate.fingerprint256.replaceAll(':',''),info.fingerprint);
  const wire=await new Promise((resolve,reject)=>{
   const connect=http.request({host:'127.0.0.1',port,method:'CONNECT',path:'api.example.test:443'});
   connect.on('error',reject);connect.on('connect',(res,socket,head)=>{
    if(res.statusCode!==200){socket.destroy();reject(Error('CONNECT '+res.statusCode));return}
    if(head.length)socket.unshift(head);
    const secure=tls.connect({socket,servername:'api.example.test',ca:certificate.toString(),ALPNProtocols:['http/1.1']},()=>{assert.equal(secure.authorized,true);secure.write('GET /secure HTTP/1.1\r\nHost: api.example.test\r\nConnection: close\r\n\r\n')});
    const chunks=[];secure.setTimeout(5000,()=>secure.destroy(Error('TLS test timeout')));secure.on('error',reject);secure.on('data',c=>chunks.push(c));secure.on('end',()=>resolve(Buffer.concat(chunks).toString()));
   });connect.end();
  });
  assert.match(wire,/HTTP\/1.1 200/);assert.match(wire,/X-Project: yes/i);assert.match(wire,/https-mock/);
  await page.screenshot({path:require('node:path').resolve(__dirname,'../.cache/https-settings-real.png'),fullPage:true});
  await page.getByRole('button',{name:'流量',exact:true}).click();await page.getByText('固定 Mock',{exact:true}).waitFor();await page.locator('tbody tr').filter({hasText:'https://api.example.test/secure'}).click();
  await page.screenshot({path:require('node:path').resolve(__dirname,'../.cache/https-flow-real.png'),fullPage:true});
  await page.getByRole('button',{name:'停止代理',exact:true}).click();await page.waitForFunction(async()=>!(await window.go.main.App.Snapshot()).proxyAddress);
  assert.equal(await page.getByRole('alert').count(),0);
  console.log('PASS: REAL CA generation/idempotence, public DER download/fingerprint, verified TLS CONNECT, default-443 HTTPS fixed Mock, project Headers and actual flow UI');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
