// Run against `wails dev` with MOCKCHARLES_DATA_DIR pointing to an empty test directory.
// No test adapter: every window.go call is handled by the real Go service.
const {chromium}=require(process.env.PLAYWRIGHT_PATH || 'playwright');
const http=require('node:http');
const assert=require('node:assert/strict');
(async()=>{
 const origin=http.createServer((req,res)=>{res.setHeader('Content-Type','application/json');res.end('{"source":"origin"}')});
 await new Promise(resolve=>origin.listen(0,'127.0.0.1',resolve));
 const target=`http://127.0.0.1:${origin.address().port}/items`;
 const browser=await chromium.launch({executablePath:process.env.EDGE_PATH || 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true});
 try {
  const page=await browser.newPage({viewport:{width:1380,height:900}});
  await page.goto('http://127.0.0.1:34115');
  await page.waitForFunction(()=>!!window.go?.main?.App);
  const initial=await page.evaluate(()=>window.go.main.App.Snapshot());
  assert.equal(initial.proxyAddress,'');
  assert.equal((initial.config.projects || []).length,0,'Use an empty isolated test data directory');
  await page.getByRole('button',{name:'＋ 项目'}).click();await page.getByLabel('项目名称').fill('HTTP 验收');await page.getByRole('button',{name:'保存项目'}).click();
  await page.getByLabel('查看项目').getByRole('option',{name:'HTTP 验收'}).waitFor({state:'attached'});
  await page.getByLabel('监听地址').fill('127.0.0.1:0');await page.getByRole('button',{name:'开启代理',exact:true}).click();
  await page.waitForFunction(async()=>!!(await window.go.main.App.Snapshot()).proxyAddress);
  const address=(await page.evaluate(()=>window.go.main.App.Snapshot())).proxyAddress;
  const request=()=>new Promise((resolve,reject)=>{http.get({host:'127.0.0.1',port:Number(address.split(':').pop()),path:target},res=>{let body='';res.on('data',x=>body+=x);res.on('end',()=>resolve({status:res.statusCode,body}))}).on('error',reject)});
  assert.equal((await request()).body,'{"source":"origin"}');
  await page.getByText('127.0.0.1',{exact:true}).first().waitFor();
  await page.getByRole('button',{name:'规则管理',exact:true}).click();await page.getByRole('button',{name:'＋ 新建接口规则'}).click();
  await page.getByLabel('名称',{exact:true}).fill('商品接口');await page.getByLabel('完整 URL（精确匹配，包含查询参数）').fill(target);await page.getByLabel('固定响应正文').fill('{"source":"mock"}');await page.getByRole('button',{name:'保存规则',exact:true}).click();
  await page.getByRole('button',{name:/商品接口/}).waitFor();
  await page.getByRole('button',{name:'规则集管理',exact:true}).click();await page.getByRole('button',{name:'＋ 新建规则集'}).click();await page.getByLabel('名称',{exact:true}).fill('验收规则集');
  await page.getByLabel('添加规则').selectOption({label:'商品接口'});await page.getByRole('button',{name:'保存编排'}).click();await page.getByRole('button',{name:/验收规则集/}).waitFor();
  await page.getByRole('button',{name:'设备管理',exact:true}).click();await page.getByText('127.0.0.1',{exact:true}).first().click();await page.getByLabel('备注',{exact:true}).fill('本机真实请求');
  const projectId=(await page.evaluate(()=>window.go.main.App.Snapshot())).config.projects[0].id;
  await page.getByLabel('绑定项目',{exact:true}).selectOption(projectId);await page.getByLabel('验收规则集',{exact:true}).check();await page.getByRole('button',{name:'保存设备配置'}).click();
  await page.getByText('本机真实请求',{exact:true}).click();await page.getByRole('button',{name:'启用所选',exact:true}).click();await page.waitForFunction(async()=>!!(await window.go.main.App.Snapshot()).config.devices[0].activeRuleSetId);
  assert.equal((await request()).body,'{"source":"mock"}');
  await page.getByRole('button',{name:'流量',exact:true}).click();await page.getByText('固定 Mock',{exact:true}).waitFor();
  await page.screenshot({path:require('node:path').resolve(__dirname,'../.cache/desktop-real-traffic.png'),fullPage:true});
  await page.getByRole('button',{name:'设备管理',exact:true}).click();await page.getByText('本机真实请求',{exact:true}).click();await page.getByRole('button',{name:'停止设备规则'}).click();await page.waitForFunction(async()=>!(await window.go.main.App.Snapshot()).config.devices[0].activeRuleSetId);
  assert.equal((await request()).body,'{"source":"origin"}');
  await page.getByRole('button',{name:'停止代理',exact:true}).click();await page.waitForFunction(async()=>!(await window.go.main.App.Snapshot()).proxyAddress);
  assert.equal(await page.getByRole('alert').count(),0);
  console.log('PASS: REAL Wails UI → SQLite project/rule/set/device saves → HTTP forwarding → fixed Mock → device stop → forwarding; real traffic rendered; listener stopped');
 }finally{await browser.close();origin.close()}
})().catch(e=>{console.error(e);process.exit(1)});

