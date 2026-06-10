import { describe, it, expect } from 'vitest'
import {
  splitTarPath,
  createTar,
  packageFilesAsTar,
  MAX_FOLDER_BYTES,
  FOLDER_TOO_LARGE_MSG,
} from './tarball'

describe('splitTarPath', () => {
  it('keeps short paths entirely in the name field', () => {
    expect(splitTarPath('board/steps/pcb/features')).toEqual({
      name: 'board/steps/pcb/features',
      prefix: '',
    })
  })

  it('accepts a path of exactly 99 chars without splitting', () => {
    const path = 'a'.repeat(99)
    expect(splitTarPath(path)).toEqual({ name: path, prefix: '' })
  })

  it('splits long paths into prefix + name on a slash boundary', () => {
    const prefix = `${'d'.repeat(70)}/${'e'.repeat(60)}` // 131 chars
    const name = 'features.txt'
    const { name: n, prefix: p } = splitTarPath(`${prefix}/${name}`)
    expect(n).toBe(name)
    expect(p).toBe(prefix)
    expect(p.length).toBeLessThanOrEqual(154)
    expect(n.length).toBeLessThanOrEqual(99)
  })

  it('round-trips: prefix + "/" + name reconstructs the original path', () => {
    const path = `${'x'.repeat(120)}/${'y'.repeat(50)}/file.dat`
    const { name, prefix } = splitTarPath(path)
    expect(`${prefix}/${name}`).toBe(path)
  })

  it('rejects paths containing null bytes', () => {
    expect(() => splitTarPath('board/fea\0tures')).toThrow(/null byte/)
  })

  it('rejects a long path with no slash to split on (would have been truncated)', () => {
    expect(() => splitTarPath('f'.repeat(150))).toThrow(/too long/)
  })

  it('rejects paths whose final segment exceeds the 99-char name field', () => {
    const path = `dir/${'f'.repeat(120)}`
    expect(() => splitTarPath(path)).toThrow(/too long/)
  })

  it('rejects paths whose earliest usable split still leaves an over-long name', () => {
    // Slash exists but only beyond position 154, so prefix can't absorb enough.
    const path = `${'a'.repeat(160)}/${'b'.repeat(20)}`
    expect(() => splitTarPath(path)).toThrow(/too long/)
  })
})

describe('createTar', () => {
  it('produces a correctly sized archive (headers + padded data + terminator)', async () => {
    const data = new Uint8Array(600).buffer // pads to 1024
    const blob = createTar([{ path: 'a/b.txt', data }])
    // 512 header + 1024 padded data + 1024 end-of-archive
    expect(blob.size).toBe(512 + 1024 + 1024)
  })

  it('throws (rather than truncating) for unarchivable paths', () => {
    const data = new Uint8Array(4).buffer
    expect(() => createTar([{ path: 'g'.repeat(200), data }])).toThrow(/too long/)
  })

  it('writes the name and ustar magic into the header', async () => {
    const data = new TextEncoder().encode('hello').buffer as ArrayBuffer
    const blob = createTar([{ path: 'hello.txt', data }])
    // jsdom's Blob has no arrayBuffer(); go through FileReader instead.
    const buf = await new Promise<ArrayBuffer>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(reader.result as ArrayBuffer)
      reader.onerror = () => reject(reader.error)
      reader.readAsArrayBuffer(blob)
    })
    const bytes = new Uint8Array(buf)
    const name = new TextDecoder().decode(bytes.slice(0, 9))
    const magic = new TextDecoder().decode(bytes.slice(257, 262))
    expect(name).toBe('hello.txt')
    expect(magic).toBe('ustar')
  })
})

describe('packageFilesAsTar', () => {
  it('rejects when the total size exceeds the cap, before reading contents', async () => {
    // Fake File objects: only `size` is consulted before the size check throws.
    const big = { size: MAX_FOLDER_BYTES + 1 } as File
    await expect(
      packageFilesAsTar([{ path: 'huge.bin', file: big }]),
    ).rejects.toThrow(FOLDER_TOO_LARGE_MSG)
  })

  it('rejects invalid paths before buffering file contents', async () => {
    const f = { size: 10 } as File // arrayBuffer intentionally missing — must not be called
    await expect(
      packageFilesAsTar([{ path: 'bad\0path', file: f }]),
    ).rejects.toThrow(/null byte/)
  })

  it('packages small files into a tar blob', async () => {
    // jsdom's File has no arrayBuffer(); a duck-typed stand-in is enough here.
    const file = { size: 10, arrayBuffer: async () => new ArrayBuffer(10) } as unknown as File
    const blob = await packageFilesAsTar([{ path: 'dir/f.bin', file }])
    // 512 header + 512 padded data + 1024 terminator
    expect(blob.size).toBe(512 + 512 + 1024)
    expect(blob.type).toBe('application/x-tar')
  })
})
