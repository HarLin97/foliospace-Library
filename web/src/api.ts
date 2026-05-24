export type Library = {
  id: number;
  name: string;
  rootPath: string;
};

export type Series = {
  id: number;
  libraryId: number;
  title: string;
  bookCount: number;
};

export type Book = {
  id: number;
  seriesId: number;
  title: string;
  format: string;
  pageCount: number;
  coverStatus: string;
  analyzed: boolean;
  filePath?: string;
};

export type Page = {
  index: number;
  name: string;
};

export type ScanJob = {
  id: number;
  libraryId: number;
  status: string;
  discoveredFiles: number;
  indexedFiles: number;
  skippedFiles: number;
  errorCount: number;
  startedAt: string;
  finishedAt?: string;
};

export type FileError = {
  id: number;
  path: string;
  code: string;
  message: string;
  lastSeen: string;
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    ...init,
  });
  if (!response.ok) {
    const body = await response.text();
    throw new Error(body || response.statusText);
  }
  return response.json() as Promise<T>;
}

export const api = {
  libraries: () => request<Library[]>("/api/libraries"),
  scan: (libraryId: number) => request<ScanJob>(`/api/libraries/${libraryId}/scan`, { method: "POST" }),
  series: () => request<Series[]>("/api/series"),
  books: (seriesId: number) => request<Book[]>(`/api/series/${seriesId}/books`),
  pages: (bookId: number) => request<Page[]>(`/api/books/${bookId}/pages`),
  jobs: () => request<ScanJob[]>("/api/jobs"),
  errors: () => request<FileError[]>("/api/errors"),
  progress: (bookId: number, pageIndex: number) =>
    request<{ ok: boolean }>(`/api/books/${bookId}/progress`, {
      method: "PUT",
      body: JSON.stringify({ pageIndex }),
    }),
};
