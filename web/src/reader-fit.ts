export type ReaderImageFitMode = "single" | "double" | "webtoon";

const fullscreenUpscaleLimit = 1.35;

export type FullscreenImageFitInput = {
  naturalWidth: number;
  naturalHeight: number;
  devicePixelRatio: number;
  mode: ReaderImageFitMode;
};

export type FullscreenImageFit = {
  maxCssWidth: number;
  maxCssHeight: number;
};

export function fullscreenImageFit({
  naturalWidth,
  naturalHeight,
  devicePixelRatio,
}: FullscreenImageFitInput): FullscreenImageFit {
  const dpr = safePositive(devicePixelRatio, 1);
  return {
    maxCssWidth: Math.max(1, Math.round(safePositive(naturalWidth, 1) * fullscreenUpscaleLimit / dpr)),
    maxCssHeight: Math.max(1, Math.round(safePositive(naturalHeight, 1) * fullscreenUpscaleLimit / dpr)),
  };
}

function safePositive(value: number, fallback: number): number {
  return Number.isFinite(value) && value > 0 ? value : fallback;
}
