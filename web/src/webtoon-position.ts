export const WEBTOON_POSITION_SCHEMA = "webtoon-position-v1";
export const DEFAULT_WEBTOON_ANCHOR_RATIO = 0.28;

export type WebtoonPosition = {
  schema: typeof WEBTOON_POSITION_SCHEMA;
  pageIndex: number;
  pageKey: string;
  pageYOffsetRatio: number;
  viewportAnchorRatio: number;
  documentProgress: number;
  pageCount: number;
  contentSignature?: string;
  updatedAt?: string;
};

export type WebtoonPageMetric = {
  index: number;
  pageKey: string;
  displayedTop: number;
  displayedHeight: number;
  logicalHeight?: number;
};

export function buildWebtoonPosition({
  pages,
  scrollTop,
  viewportHeight,
  viewportAnchorRatio = DEFAULT_WEBTOON_ANCHOR_RATIO,
}: {
  pages: WebtoonPageMetric[];
  scrollTop: number;
  viewportHeight: number;
  viewportAnchorRatio?: number;
}): WebtoonPosition {
  const normalizedAnchor = clampUnit(viewportAnchorRatio);
  const anchorY = scrollTop + viewportHeight * normalizedAnchor;
  const page = findAnchorPage(pages, anchorY);
  const pageYOffsetRatio = page ? clampUnit((anchorY - page.displayedTop) / safePositive(page.displayedHeight, 1)) : 0;
  const documentProgress = page ? documentProgressForPage(pages, page.index, pageYOffsetRatio) : 0;
  return {
    schema: WEBTOON_POSITION_SCHEMA,
    pageIndex: page?.index ?? 0,
    pageKey: page?.pageKey ?? "",
    pageYOffsetRatio,
    viewportAnchorRatio: normalizedAnchor,
    documentProgress,
    pageCount: pages.length,
  };
}

export function resolveWebtoonRestoreTarget({
  position,
  pages,
  viewportHeight,
}: {
  position: WebtoonPosition;
  pages: WebtoonPageMetric[];
  viewportHeight: number;
}): { pageIndex: number; scrollTop: number } | null {
  if (pages.length === 0) return null;
  const page = findRestorePage(position, pages);
  if (!page) return null;
  const pageYOffsetRatio = restoreYOffsetRatio(position, page, pages);
  const anchorRatio = position.viewportAnchorRatio > 0 ? clampUnit(position.viewportAnchorRatio) : DEFAULT_WEBTOON_ANCHOR_RATIO;
  return {
    pageIndex: page.index,
    scrollTop: Math.max(0, Math.round(page.displayedTop + page.displayedHeight * pageYOffsetRatio - viewportHeight * anchorRatio)),
  };
}

export function stabilizeWebtoonDocumentProgress(
  position: WebtoonPosition,
  previousPosition: WebtoonPosition | null | undefined,
  pages: WebtoonPageMetric[],
): WebtoonPosition {
  if (hasCompleteLogicalHeights(pages) || !previousPosition) return position;
  const previousProgress = clampUnit(previousPosition.documentProgress);
  const pageCount = safePositive(position.pageCount, safePositive(previousPosition.pageCount, pages.length));
  if (pageCount <= 0 || position.pageIndex < 0 || previousPosition.pageIndex < 0) {
    return { ...position, documentProgress: previousProgress };
  }
  const deltaPages = position.pageIndex - previousPosition.pageIndex + clampUnit(position.pageYOffsetRatio) - clampUnit(previousPosition.pageYOffsetRatio);
  return { ...position, documentProgress: clampUnit(previousProgress + deltaPages / pageCount) };
}

function findAnchorPage(pages: WebtoonPageMetric[], anchorY: number): WebtoonPageMetric | null {
  if (pages.length === 0) return null;
  for (const page of pages) {
    if (anchorY >= page.displayedTop && anchorY <= page.displayedTop + page.displayedHeight) {
      return page;
    }
  }
  const sorted = [...pages].sort((left, right) => left.displayedTop - right.displayedTop);
  const first = sorted[0];
  const last = sorted[sorted.length - 1];
  if (!first || !last) return null;
  if (anchorY < first.displayedTop) return first;
  return last;
}

function findRestorePage(position: WebtoonPosition, pages: WebtoonPageMetric[]): WebtoonPageMetric | null {
  const byKey = position.pageKey ? pages.find((page) => page.pageKey === position.pageKey) : null;
  if (byKey) return byKey;
  const byIndex = pages.find((page) => page.index === position.pageIndex);
  if (byIndex) return byIndex;
  return pageFromDocumentProgress(pages, position.documentProgress);
}

function restoreYOffsetRatio(position: WebtoonPosition, page: WebtoonPageMetric, pages: WebtoonPageMetric[]): number {
  if (position.pageKey === page.pageKey || position.pageIndex === page.index) {
    return clampUnit(position.pageYOffsetRatio);
  }
  return pageRatioFromDocumentProgress(pages, page.index, position.documentProgress);
}

function documentProgressForPage(pages: WebtoonPageMetric[], pageIndex: number, pageYOffsetRatio: number): number {
  const total = totalLogicalHeight(pages);
  if (total <= 0) return 0;
  let before = 0;
  for (const page of pages) {
    if (page.index === pageIndex) {
      return clampUnit((before + logicalHeight(page) * clampUnit(pageYOffsetRatio)) / total);
    }
    before += logicalHeight(page);
  }
  return 0;
}

function pageFromDocumentProgress(pages: WebtoonPageMetric[], documentProgress: number): WebtoonPageMetric | null {
  const total = totalLogicalHeight(pages);
  const target = total * clampUnit(documentProgress);
  let before = 0;
  for (const page of pages) {
    const height = logicalHeight(page);
    if (target < before + height) {
      return page;
    }
    before += height;
  }
  return pages[pages.length - 1] ?? null;
}

function pageRatioFromDocumentProgress(pages: WebtoonPageMetric[], pageIndex: number, documentProgress: number): number {
  const total = totalLogicalHeight(pages);
  const target = total * clampUnit(documentProgress);
  let before = 0;
  for (const page of pages) {
    const height = logicalHeight(page);
    if (page.index === pageIndex) {
      return clampUnit((target - before) / safePositive(height, 1));
    }
    before += height;
  }
  return 0;
}

function totalLogicalHeight(pages: WebtoonPageMetric[]): number {
  return pages.reduce((sum, page) => sum + logicalHeight(page), 0);
}

function hasCompleteLogicalHeights(pages: WebtoonPageMetric[]): boolean {
  return pages.length > 0 && pages.every((page) => safePositive(page.logicalHeight, 0) > 0);
}

function logicalHeight(page: WebtoonPageMetric): number {
  return safePositive(page.logicalHeight, safePositive(page.displayedHeight, 1));
}

function safePositive(value: number | undefined, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : fallback;
}

function clampUnit(value: number | undefined): number {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) return 0;
  if (value > 1) return 1;
  return value;
}
