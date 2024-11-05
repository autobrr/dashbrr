import sharp from "sharp";
import { readFileSync } from "fs";
import { join } from "path";

const sizes = [192, 512];
const inputSvg = readFileSync(join(process.cwd(), "src/assets/logo.svg"));

async function generateIcons() {
  try {
    // Generate PWA icons
    for (const size of sizes) {
      await sharp(inputSvg)
        .resize(size, size)
        .png()
        .toFile(join(process.cwd(), `public/pwa-${size}x${size}.png`));
    }

    // Generate apple-touch-icon (180x180 is standard for Apple)
    await sharp(inputSvg)
      .resize(180, 180)
      .png()
      .toFile(join(process.cwd(), "public/apple-touch-icon.png"));

    console.log("PWA icons generated successfully!");
  } catch (error) {
    console.error("Error generating PWA icons:", error);
  }
}

generateIcons();
