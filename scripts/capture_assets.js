const puppeteer = require('puppeteer-core');
const path = require('path');
const fs = require('fs');
const { execSync } = require('child_process');

const CHROME_PATH = 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
const OUTPUT_DIR = path.join(__dirname, '..', 'docs', 'images');
const FRAMES_DIR = path.join(__dirname, '..', 'docs', 'images', 'gif_frames');

if (!fs.existsSync(OUTPUT_DIR)) {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true });
}
if (!fs.existsSync(FRAMES_DIR)) {
  fs.mkdirSync(FRAMES_DIR, { recursive: true });
}

async function capture() {
  console.log('Launching Chrome via puppeteer-core...');
  const browser = await puppeteer.launch({
    executablePath: CHROME_PATH,
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
  });

  const page = await browser.newPage();

  // 1. Desktop Screenshot (1440x960, scale 2 for retina clarity)
  console.log('Capturing Desktop Dashboard (1440x960)...');
  await page.setViewport({ width: 1440, height: 960, deviceScaleFactor: 2 });
  await page.goto('http://localhost:8474', { waitUntil: 'domcontentloaded', timeout: 15000 });
  // Wait 3 seconds for live SSE telemetry to populate sparklines and metrics
  await new Promise((r) => setTimeout(r, 3000));

  const desktopPath = path.join(OUTPUT_DIR, 'faultcraft_dashboard.png');
  await page.screenshot({ path: desktopPath, fullPage: false });
  console.log('✓ Saved:', desktopPath);

  // 2. Mobile Screenshot (390x844, scale 2)
  console.log('Capturing Mobile Viewport (390x844)...');
  await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2 });
  await new Promise((r) => setTimeout(r, 1000));

  const mobilePath = path.join(OUTPUT_DIR, 'faultcraft_mobile.png');
  await page.screenshot({ path: mobilePath, fullPage: false });
  console.log('✓ Saved:', mobilePath);

  // 3. Traffic Inspector Focused Screenshot (Desktop, scrolled to Inspector)
  console.log('Capturing Inspector Screenshot...');
  await page.setViewport({ width: 1440, height: 960, deviceScaleFactor: 2 });
  await new Promise((r) => setTimeout(r, 1000));

  // Scroll down slightly so inspector and proxy cards are centered
  await page.evaluate(() => {
    window.scrollTo({ top: 320, behavior: 'instant' });
  });
  await new Promise((r) => setTimeout(r, 800));

  const inspectorPath = path.join(OUTPUT_DIR, 'faultcraft_inspector.png');
  await page.screenshot({ path: inspectorPath });
  console.log('✓ Saved:', inspectorPath);

  // 4. Capture Frame-by-Frame for Animated Demo GIF
  console.log('Recording interactive frames for demo GIF...');
  await page.evaluate(() => {
    window.scrollTo({ top: 0, behavior: 'instant' });
  });
  await new Promise((r) => setTimeout(r, 500));

  // Clean old frames
  fs.readdirSync(FRAMES_DIR).forEach((f) => fs.unlinkSync(path.join(FRAMES_DIR, f)));

  let frameCount = 0;
  const captureFrame = async () => {
    const filename = path.join(FRAMES_DIR, `frame_${String(frameCount).padStart(4, '0')}.png`);
    await page.screenshot({ path: filename });
    frameCount++;
  };

  // Phase 1: View initial active dashboard (10 frames = ~1 sec at 10fps)
  for (let i = 0; i < 10; i++) {
    await captureFrame();
    await new Promise((r) => setTimeout(r, 100));
  }

  // Phase 2: Click on 1-Click Chaos Preset "Cellular 3G" or "Flaky Database"
  console.log('Triggering preset click in demo recording...');
  const presetButtons = await page.$$('button');
  for (const btn of presetButtons) {
    const text = await page.evaluate((el) => el.innerText, btn);
    if (text.includes('Flaky Database') || text.includes('Cellular 3G')) {
      await btn.click();
      break;
    }
  }

  // Capture transition and toast notification (15 frames = ~1.5 sec)
  for (let i = 0; i < 15; i++) {
    await captureFrame();
    await new Promise((r) => setTimeout(r, 100));
  }

  // Phase 3: Scroll smoothly down to L7 Traffic Inspector (10 frames)
  for (let i = 1; i <= 10; i++) {
    await page.evaluate((step) => {
      window.scrollTo({ top: step * 35, behavior: 'instant' });
    }, i);
    await captureFrame();
    await new Promise((r) => setTimeout(r, 100));
  }

  // Phase 4: Watch live frames stream through (15 frames = ~1.5 sec)
  for (let i = 0; i < 15; i++) {
    await captureFrame();
    await new Promise((r) => setTimeout(r, 100));
  }

  await browser.close();
  console.log(`Captured ${frameCount} frames for demo GIF.`);

  // 5. Compile GIF using ffmpeg
  const gifPath = path.join(OUTPUT_DIR, 'faultcraft_demo.gif');
  console.log('Encoding optimized animated GIF via ffmpeg...');

  // Use palettegen & paletteuse for ultra-crisp colors without dithering artifacts
  const inputPattern = path.join(FRAMES_DIR, 'frame_%04d.png');
  const palettePath = path.join(FRAMES_DIR, 'palette.png');

  try {
    execSync(`ffmpeg -y -framerate 10 -i "${inputPattern}" -vf "fps=10,scale=1000:-1:flags=lanczos,palettegen=max_colors=128" "${palettePath}"`, { stdio: 'inherit' });
    execSync(`ffmpeg -y -framerate 10 -i "${inputPattern}" -i "${palettePath}" -lavfi "fps=10,scale=1000:-1:flags=lanczos [x]; [x][1:v] paletteuse=dither=bayer:bayer_scale=3" "${gifPath}"`, { stdio: 'inherit' });
    console.log('✓ Successfully created animated GIF:', gifPath);

    // Clean up temporary frames
    fs.readdirSync(FRAMES_DIR).forEach((f) => fs.unlinkSync(path.join(FRAMES_DIR, f)));
    fs.rmdirSync(FRAMES_DIR);
  } catch (err) {
    console.error('Error generating GIF with palette, trying fallback:', err);
    execSync(`ffmpeg -y -framerate 10 -i "${inputPattern}" -vf "scale=960:-1" "${gifPath}"`, { stdio: 'inherit' });
  }

  console.log('All visual assets captured successfully!');
}

capture().catch((err) => {
  console.error('Capture failed:', err);
  process.exit(1);
});
