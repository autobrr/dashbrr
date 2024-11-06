import sharp from "sharp";
import { readFileSync } from "fs";
import { join } from "path";

const sizes = [192, 512];
const inputSvg = readFileSync(join(process.cwd(), "src/assets/logo.svg"));

async function generateIcons() {
  try {
    for (const size of sizes) {
      // Create a dark background with correct color #18181B
      const background = await sharp({
        create: {
          width: size,
          height: size,
          channels: 4,
          background: { r: 24, g: 24, b: 27, alpha: 1 }, // #18181B
        },
      })
        .png()
        .toBuffer();

      // Resize the logo
      const resizedLogo = await sharp(inputSvg)
        .resize(Math.round(size * 0.7), Math.round(size * 0.7)) // Make logo slightly smaller than background
        .toBuffer();

      // Composite them together
      await sharp(background)
        .composite([
          {
            input: resizedLogo,
            blend: "over",
            gravity: "center",
          },
        ])
        .png()
        .toFile(join(process.cwd(), `public/pwa-${size}x${size}.png`));
    }

    // Generate apple-touch-icon (180x180 is standard for Apple)
    const size = 180;
    const background = await sharp({
      create: {
        width: size,
        height: size,
        channels: 4,
        background: { r: 24, g: 24, b: 27, alpha: 1 }, // #18181B
      },
    })
      .png()
      .toBuffer();

    const resizedLogo = await sharp(inputSvg)
      .resize(Math.round(size * 0.7), Math.round(size * 0.7))
      .toBuffer();

    await sharp(background)
      .composite([
        {
          input: resizedLogo,
          blend: "over",
          gravity: "center",
        },
      ])
      .png()
      .toFile(join(process.cwd(), "public/apple-touch-icon.png"));

    console.log("PWA icons generated successfully!");
  } catch (error) {
    console.error("Error generating PWA icons:", error);
  }
}

generateIcons();
