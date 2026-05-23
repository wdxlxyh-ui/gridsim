async function run() {
  const { chromium } = require('/root/.npm/_npx/e41f203b7505f1fb/node_modules/playwright');
  const browser = await chromium.launch({
    executablePath: '/root/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',
    headless: true,
  });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();

  const BASE = 'http://localhost:8989';
  const out = 'docs/screenshots/';

  // Login page
  await page.goto(BASE + '/login', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: out + 'login.png' });
  console.log('OK login.png');

  // Fill and submit login
  await page.fill('input[type="text"], input[name="username"]', 'admin');
  await page.fill('input[type="password"], input[name="password"]', 'admin');
  const btn = await page.$('button');
  if (btn) await btn.click();
  await page.waitForTimeout(2000);

  // Config page
  await page.goto(BASE + '/config', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  await page.screenshot({ path: out + 'config-page.png' });
  console.log('OK config-page.png');

  // Monitor page
  await page.goto(BASE + '/monitor', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  await page.screenshot({ path: out + 'monitor-page.png' });
  console.log('OK monitor-page.png');

  // Try detail page
  await page.goto(BASE + '/config', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);
  const links = await page.$$('a');
  let detailUrl = null;
  for (const link of links) {
    const href = await link.getAttribute('href');
    if (href && href.includes('/instance/')) {
      detailUrl = href;
      break;
    }
  }
  if (detailUrl) {
    await page.goto(BASE + detailUrl, { waitUntil: 'networkidle' });
    await page.waitForTimeout(2500);
    await page.screenshot({ path: out + 'detail-page.png' });
    console.log('OK detail-page.png');

    // Trend page
    const trendLink = await page.$('a[href*="/trend/"]');
    if (trendLink) {
      const thref = await trendLink.getAttribute('href');
      await page.goto(BASE + thref, { waitUntil: 'networkidle' });
      await page.waitForTimeout(2000);
      await page.screenshot({ path: out + 'trend-page.png' });
      console.log('OK trend-page.png');
    }
  }

  await browser.close();
  console.log('DONE');
}

run().catch(e => { console.error(e); process.exit(1); });
