/**
 * Minimal browser-side tar archive builder.
 *
 * Takes a list of files with relative paths and produces a tar Blob.
 * Used to package a dropped ODB++ folder into a tar archive before
 * uploading to S3, so users don't have to tar/tgz their files manually.
 *
 * Implements the POSIX ustar tar format (512-byte headers + padded data).
 * No compression — the files are typically small enough (<50 MB) that
 * the upload time difference is negligible and avoiding compression
 * keeps the code simple and fast.
 */

/**
 * Hard cap on total folder content. Everything is buffered in memory before
 * upload, and the upload path itself rejects payloads this large anyway.
 */
export const MAX_FOLDER_BYTES = 500 * 1024 * 1024 // 500 MB

export const FOLDER_TOO_LARGE_MSG =
  'Folder exceeds 500 MB — compress it and upload as an archive instead.'

interface TarEntry {
  /** Relative path inside the archive (e.g. "my-board/steps/pcb/layers/..."). */
  path: string
  /** File data as an ArrayBuffer. */
  data: ArrayBuffer
}

/** Encode a string into a fixed-length Uint8Array, null-terminated. */
function encodeString(str: string, len: number): Uint8Array<ArrayBuffer> {
  const buf = new Uint8Array(len) as Uint8Array<ArrayBuffer>
  const encoder = new TextEncoder()
  const encoded = encoder.encode(str.slice(0, len - 1))
  buf.set(encoded)
  return buf
}

/** Encode a number as a zero-padded octal string of `len` bytes (with trailing null). */
function encodeOctal(value: number, len: number): Uint8Array<ArrayBuffer> {
  const str = value.toString(8).padStart(len - 1, '0')
  return encodeString(str, len)
}

/**
 * Validate a path and split it into the ustar name (<=99 chars) and prefix
 * (<=154 chars) fields. Throws a clear error instead of silently truncating
 * paths that don't fit, and rejects null bytes (which would corrupt the
 * null-terminated header fields).
 *
 * Exported for tests.
 */
export function splitTarPath(path: string): { name: string; prefix: string } {
  if (path.includes('\0')) {
    throw new Error(`Invalid file path in folder (contains a null byte): "${path.replace(/\0/g, '\\0')}"`)
  }
  // encodeString reserves one byte for the null terminator, so the usable
  // capacity is name <= 99 and prefix <= 154.
  if (path.length <= 99) return { name: path, prefix: '' }
  const split = path.lastIndexOf('/', 154)
  if (split > 0) {
    const prefix = path.slice(0, split)
    const name = path.slice(split + 1)
    if (name.length <= 99) return { name, prefix }
  }
  throw new Error(
    `File path is too long to archive: "${path}". ` +
      'Shorten the folder or file names, or compress the folder yourself and upload the archive instead.',
  )
}

/** Build a 512-byte ustar header for a single file entry. */
function buildHeader(entry: TarEntry): Uint8Array<ArrayBuffer> {
  const header = new Uint8Array(512) as Uint8Array<ArrayBuffer>

  // For paths > 99 chars, split into prefix (154) + name (99). Throws on
  // paths that don't fit rather than silently truncating.
  const { name, prefix } = splitTarPath(entry.path)

  header.set(encodeString(name, 100), 0)          // name
  header.set(encodeOctal(0o644, 8), 100)           // mode
  header.set(encodeOctal(0, 8), 108)               // uid
  header.set(encodeOctal(0, 8), 116)               // gid
  header.set(encodeOctal(entry.data.byteLength, 12), 124) // size
  header.set(encodeOctal(Math.floor(Date.now() / 1000), 12), 136) // mtime
  // checksum placeholder — 8 spaces
  header.set(new Uint8Array([0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20]), 148)
  header[156] = 0x30 // typeflag '0' = regular file
  // linkname: 100 bytes at 157 (zeros = no link)
  header.set(encodeString('ustar', 6), 257)        // magic
  header.set(encodeString('00', 2), 263)            // version
  header.set(encodeString('', 32), 265)             // uname
  header.set(encodeString('', 32), 297)             // gname
  header.set(encodeOctal(0, 8), 329)                // devmajor
  header.set(encodeOctal(0, 8), 337)                // devminor
  header.set(encodeString(prefix, 155), 345)        // prefix

  // Compute checksum: unsigned sum of all 512 bytes (with checksum field as spaces).
  let chksum = 0
  for (let i = 0; i < 512; i++) chksum += header[i]
  header.set(encodeOctal(chksum, 7), 148)
  header[155] = 0x20 // trailing space after checksum (traditional format)

  return header
}

/**
 * Create a tar archive Blob from a list of entries.
 *
 * The archive ends with two 512-byte zero blocks as required by the format.
 */
export function createTar(entries: TarEntry[]): Blob {
  const parts: BlobPart[] = []

  for (const entry of entries) {
    parts.push(buildHeader(entry))
    parts.push(entry.data)
    // Pad data to a multiple of 512 bytes.
    const remainder = entry.data.byteLength % 512
    if (remainder > 0) {
      parts.push(new Uint8Array(512 - remainder) as Uint8Array<ArrayBuffer>)
    }
  }

  // End-of-archive marker: two 512-byte zero blocks.
  parts.push(new Uint8Array(1024) as Uint8Array<ArrayBuffer>)

  return new Blob(parts, { type: 'application/x-tar' })
}

// ── Folder reading utilities ─────────────────────────────────────────────────

/**
 * Recursively read all files from a `FileSystemDirectoryEntry` (drag-and-drop API).
 * Returns a flat list of `{ path, file }` where `path` is the relative path
 * from the drop root (e.g. "my-board/steps/pcb/layers/l01_top/features").
 *
 * Aborts with a clear error as soon as the accumulated file size exceeds
 * MAX_FOLDER_BYTES, before everything gets buffered into memory.
 */
export async function readDirectoryEntry(
  dirEntry: FileSystemDirectoryEntry,
  basePath = '',
  totals: { bytes: number } = { bytes: 0 },
): Promise<{ path: string; file: File }[]> {
  const results: { path: string; file: File }[] = []
  const prefix = basePath ? `${basePath}/${dirEntry.name}` : dirEntry.name

  const reader = dirEntry.createReader()
  // readEntries returns at most 100 entries at a time in some browsers,
  // so we call it repeatedly until it returns an empty array.
  let batch: FileSystemEntry[] = []
  do {
    batch = await new Promise<FileSystemEntry[]>((resolve, reject) =>
      reader.readEntries(resolve, (err) =>
        reject(
          new Error(
            `Could not read folder "${prefix}" — the browser was denied access. ` +
              `Check the folder's permissions, or compress it and upload the archive instead. (${err?.name ?? 'unknown error'})`,
          ),
        ),
      ),
    )
    for (const entry of batch) {
      if (entry.isFile) {
        const fileEntry = entry as FileSystemFileEntry
        const file = await new Promise<File>((resolve, reject) =>
          fileEntry.file(resolve, (err) =>
            reject(
              new Error(
                `Could not read file "${prefix}/${entry.name}" — it may be locked by another program or you may not have permission to access it. (${err?.name ?? 'unknown error'})`,
              ),
            ),
          ),
        )
        totals.bytes += file.size
        if (totals.bytes > MAX_FOLDER_BYTES) throw new Error(FOLDER_TOO_LARGE_MSG)
        results.push({ path: `${prefix}/${entry.name}`, file })
      } else if (entry.isDirectory) {
        const subResults = await readDirectoryEntry(
          entry as FileSystemDirectoryEntry,
          prefix,
          totals,
        )
        results.push(...subResults)
      }
    }
  } while (batch.length > 0)

  return results
}

/**
 * Package a list of files (with relative paths) into a tar Blob.
 *
 * Accepts either:
 * - The output of `readDirectoryEntry` (drag-and-drop)
 * - A `FileList` from an `<input webkitdirectory>` element (uses `webkitRelativePath`)
 *
 * Validates total size and every path BEFORE buffering file contents, so
 * oversized folders and unarchivable paths fail fast with a clear message.
 */
export async function packageFilesAsTar(
  files: { path: string; file: File }[] | FileList,
): Promise<Blob> {
  let entries: { path: string; file: File }[]

  if (files instanceof FileList) {
    entries = Array.from(files).map((f) => ({
      path: (f as File & { webkitRelativePath?: string }).webkitRelativePath || f.name,
      file: f,
    }))
  } else {
    entries = files
  }

  const totalBytes = entries.reduce((sum, e) => sum + e.file.size, 0)
  if (totalBytes > MAX_FOLDER_BYTES) throw new Error(FOLDER_TOO_LARGE_MSG)

  // Validate every path up front so we fail before reading file contents.
  for (const { path } of entries) splitTarPath(path)

  const tarEntries: TarEntry[] = await Promise.all(
    entries.map(async ({ path, file }) => ({
      path,
      data: await file.arrayBuffer().catch(() => {
        throw new Error(
          `Could not read file "${path}" — it may have been moved, locked by another program, or you may not have permission to access it.`,
        )
      }),
    })),
  )

  return createTar(tarEntries)
}
