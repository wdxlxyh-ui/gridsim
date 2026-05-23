import { chromium } from 'playwright';
import { fileURLToPath } from 'url';
import path from 'path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.resolve(__dirname, '..', 'docs', 'screenshots');

const BASE = 'http://localhost:8989';

async function main() {
  const browser = await chromium.launch({
    executablePath: '/root/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome',
    headless: true,
  });

  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
  });
  const page = await context.newPage();

  // 1. Login page
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: `${outDir}/login.png`, fullPage: false });
  console.log('✅ login.png');

  // 2. Login
  await page.fill('input[type="text"], input[name="username"], input[placeholder*="用户"]', 'admin');
  await page.fill('input[type="password"], input[name="password"]', 'admin');
  // Try clicking login button
  const loginBtn = await page.$('button:has-text("登录"), button[type="submit"]');
  if (loginBtn) await loginBtn.click();
  await page.waitForTimeout(2000);

  // 3. ConfigPage (instance list)
  await page.goto(`${BASE}/config`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${outDir}/config-page.png`, fullPage: false });
  console.log('✅ config-page.png');

  // 4. MonitorPage
  await page.goto(`${BASE}/monitor`, { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: `${outDir}/monitor-page.png`, fullPage: false });
  console.log('✅ monitor-page.png');

  // 5. DetailPage - navigate to first instance if exists
  const instanceLink = await page.$('a[href*="/instance/"]');
  if (instanceLink) {
    const href = await instanceLink.getAttribute('href');
    await page.goto(`${BASE}${href}`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(2000);
    await page.screenshot({ path: `${outDir}/detail-page.png`, fullPage: false });
    console.log('✅ detail-page.png');

    // 6. TrendPage
    const trendLink = await page.$('a[href*="/trend/"]');
    if (trendLink) {
      const thref = await trendLink.getAttribute('href');
      await page.goto(`${BASE}${thref}`, { waitUntil: 'networkidle' });
      await page.waitForTimeout(2000);
      await page.screenshot({ path: `${outDir}/trend-page.png`, fullPage: false });
      console.log('✅ trend-page.png');
    }
  }

  await browser.close();
  console.log('🎉 All screenshots done!');
}

main().catch(err => { console.error(err); process.exit(1); });
