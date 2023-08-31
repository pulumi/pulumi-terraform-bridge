output "files" {
  value = files
}

output "filesOnDisk" {
  value = { for p, f in files : p => f if f.sourcePath != null }
}

output "filesInMemory" {
  value = { for p, f in files : p => f if f.content != null }
}
