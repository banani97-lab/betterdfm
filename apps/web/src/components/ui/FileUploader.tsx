'use client'

import { useCallback, useState } from 'react'
import { Upload, File, X } from 'lucide-react'
import { cn } from '@/lib/utils'

interface FileUploaderProps {
  accept?: string
  onFile: (file: File) => void
  disabled?: boolean
}

export function FileUploader({ accept, onFile, disabled }: FileUploaderProps) {
  const [dragging, setDragging] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)

  const handleFile = useCallback(
    (file: File) => {
      setSelectedFile(file)
      onFile(file)
    },
    [onFile]
  )

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setDragging(false)
      const file = e.dataTransfer.files[0]
      if (file) handleFile(file)
    },
    [handleFile]
  )

  const onInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
  }

  const clearFile = (e: React.MouseEvent) => {
    e.stopPropagation()
    setSelectedFile(null)
  }

  return (
    <div
      onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
      onDragLeave={() => setDragging(false)}
      onDrop={onDrop}
      className={cn(
        'relative flex flex-col items-center justify-center w-full h-48 border-2 border-dashed rounded-lg cursor-pointer transition-colors',
        dragging ? 'border-primary bg-primary/5' : 'border-gray-300 hover:border-gray-400 bg-gray-50',
        disabled && 'pointer-events-none opacity-50'
      )}
    >
      <input
        type="file"
        accept={accept}
        onChange={onInputChange}
        className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
        disabled={disabled}
      />
      {selectedFile ? (
        <div className="flex items-center gap-3 px-4">
          <File className="h-8 w-8 text-primary flex-shrink-0" />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-gray-900 truncate">{selectedFile.name}</p>
            <p className="text-xs text-gray-500">{(selectedFile.size / 1024).toFixed(1)} KB</p>
          </div>
          <button
            onClick={clearFile}
            className="p-1 rounded hover:bg-gray-200 text-gray-500"
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ) : (
        <div className="flex flex-col items-center gap-2 text-center px-4">
          <Upload className="h-10 w-10 text-gray-400" />
          <div>
            <p className="text-sm font-medium text-gray-700">Drop your Gerber or ODB++ file here</p>
            <p className="text-xs text-gray-500 mt-1">or click to browse — .zip, .gbr, .ger accepted</p>
          </div>
        </div>
      )}
    </div>
  )
}
