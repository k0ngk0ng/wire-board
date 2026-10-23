// WebKit-specific supplement: agent-browser's installed CLI exposes Chrome,
// not desktop WebKit. Keep the browser, profile and caches in this workspace.
import { createInterface } from 'node:readline';
import { pathToFileURL } from 'node:url';
const { webkit } = await import(pathToFileURL(process.env.POKER_PLAYWRIGHT_MODULE).href);
const context = await webkit.launchPersistentContext(process.env.AGENT_BROWSER_PROFILE, {headless:true,viewport:{width:390,height:720},isMobile:true,hasTouch:true});
const page=context.pages()[0];page.setDefaultTimeout(20000);
const errors=[];page.on('pageerror',e=>errors.push(e.message));
for await (const line of createInterface({input:process.stdin})) {
 const c=JSON.parse(line);let data={};
 try {
  if(c[0]==='open')await page.goto(c[1]);
  else if(c[0]==='eval')data={result:(await page.evaluate(c[1]))??null};
  else if(c[0]==='wait') {if(c[1]==='--fn')await page.waitForFunction(c[2]);else await page.locator(c[1]).first().waitFor();}
  else if(c[0]==='set')await page.setViewportSize({width:+c[2],height:+c[3]});
  else if(c[0]==='tap')await page.touchscreen.tap(+c[1],+c[2]);
  else if(c[0]==='mouse') {if(c[1]==='move')await page.mouse.move(+c[2],+c[3]);else if(c[1]==='down')await page.mouse.down();else await page.mouse.up();}
  else if(c[0]==='find')await page.getByRole(c[2],{name:c[c.indexOf('--name')+1],exact:true}).click();
  else if(c[0]==='click')await page.locator(c[1]).click();
  else if(c[0]==='focus')await page.locator(c[1]).first().focus();
  else if(c[0]==='press')await page.keyboard.press(c[1]);
  else if(c[0]==='screenshot')await page.screenshot({path:c[1],fullPage:c.includes('--full')});
  else if(c[0]==='errors')data={errors};
  else if(c[0]==='close')await context.close();
  else throw Error('Unsupported WebKit verification command: '+c[0]);
  process.stdout.write(JSON.stringify({success:true,data})+'\n');
 } catch(e) {process.stdout.write(JSON.stringify({success:false,error:String(e)})+'\n');}
 if(c[0]==='close')break;
}
