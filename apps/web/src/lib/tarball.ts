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

interface TarEntry {
  /** Relative path inside the archive (e.g. "my-board/steps/pcb/layers/..."). */
  path: string
  /** File data as an ArrayBuffer. */
  data: ArrayBuffer
}

/** Encode a string into a fixed-length Uint8Array, null-terminated. */
function encodeString(str: string, len: number): Uint8Array {
  const buf = new Uint8Array(len)
  const encoder = new TextEncoder()
  const encoded = encoder.encode(str.slice(0, len - 1))
  buf.set(encoded)
  return buf
}

/** Encode a number as a zero-padded octal string of `len` bytes (with trailing null). */
function encodeOctal(value: number, len: number): Uint8Array {
  const str = value.toString(8).padStart(len - 1, '0')
  return encodeString(str, len)
}

/** Build a 512-byte ustar header for a single file entry. */
function buildHeader(entry: TarEntry): Uint8Array {
  const header = new Uint8Array(512)

  // For paths > 100 chars, split into prefix (155) + name (100).
  let name = entry.path
  let prefix = ''
  if (name.length > 100) {
    const split = name.lastIndexOf('/', 155)
    if (split > 0) {
      prefix = name.slice(0, split)
      name = name.slice(split + 1)
    }
  }

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
  const parts: (Uint8Array | ArrayBuffer)[] = []

  for (const entry of entries) {
    parts.push(buildHeader(entry))
    parts.push(entry.data)
    // Pad data to a multiple of 512 bytes.
    const remainder = entry.data.byteLength % 512
    if (remainder > 0) {
      parts.push(new Uint8Array(512 - remainder))
    }
  }

  // End-of-archive marker: two 512-byte zero blocks.
  parts.push(new Uint8Array(1024))

  return new Blob(parts, { type: 'application/x-tar' })
}

// ── Folder reading utilities ─────────────────────────────────────────────────

/**
 * Recursively read all files from a `FileSystemDirectoryEntry` (drag-and-drop API).
 * Returns a flat list of `{ path, file }` where `path` is the relative path
 * from the drop root (e.g. "my-board/steps/pcb/layers/l01_top/features").
 */
export async function readDirectoryEntry(
  dirEntry: FileSystemDirectoryEntry,
  basePath = '',
): Promise<{ path: string; file: File }[]> {
  const results: { path: string; file: File }[] = []
  const prefix = basePath ? `${basePath}/${dirEntry.name}` : dirEntry.name

  const reader = dirEntry.createReader()
  // readEntries returns at most 100 entries at a time in some browsers,
  // so we call it repeatedly until it returns an empty array.
  let batch: FileSystemEntry[] = []
  do {
    batch = await new Promise<FileSystemEntry[]>((resolve, reject) =>
      reader.readEntries(resolve, reject),
    )
    for (const entry of batch) {
      if (entry.isFile) {
        const fileEntry = entry as FileSystemFileEntry
        const file = await new Promise<File>((resolve, reject) =>
          fileEntry.file(resolve, reject),
        )
        results.push({ path: `${prefix}/${entry.name}`, file })
      } else if (entry.isDirectory) {
        const subResults = await readDirectoryEntry(
          entry as FileSystemDirectoryEntry,
          prefix,
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

  const tarEntries: TarEntry[] = await Promise.all(
    entries.map(async ({ path, file }) => ({
      path,
      data: await file.arrayBuffer(),
    })),
  )

  return createTar(tarEntries)
}
