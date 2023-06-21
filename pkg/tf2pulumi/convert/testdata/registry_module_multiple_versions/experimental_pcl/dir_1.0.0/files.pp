allFilePaths    = notImplemented("fileset(var.base_dir,\"**\")")
staticFilePaths = notImplemented("toset([\nforpinlocal.all_file_paths:p\niflength(p)<length(var.template_file_suffix)||substr(p,length(p)-length(var.template_file_suffix),length(var.template_file_suffix))!=var.template_file_suffix\n])")
templateFilePaths = { for p in allFilePaths : invoke("std:index:substr", {
  input  = p
  length = 0
  offset = length(p) - length(templateFileSuffix)
}).result => p if !notImplemented("contains(local.static_file_paths,p)") }
templateFileContents = { for p, sp in templateFilePaths : p => notImplemented("templatefile(\"$${var.base_dir}/$${sp}\",var.template_vars)") }
staticFileLocalPaths = { for p in staticFilePaths : p => "${baseDir}/${p}" }
outputFilePaths      = notImplemented("setunion(keys(local.template_file_paths),local.static_file_paths)")
fileSuffixMatches    = { for p in outputFilePaths : p => notImplemented("regexall(\"\\\\.[^\\\\.]+\\\\z\",p)") }
fileSuffixes         = { for p, ms in fileSuffixMatches : p => length(ms) > 0 ? ms[0] : "" }
myfileTypes          = { for p in outputFilePaths : p => notImplemented("lookup(var.file_types,local.file_suffixes[p],var.default_file_type)") }
files                = notImplemented("merge(\n{\nforpinkeys(local.template_file_paths):p=>{\ncontent_type=local.file_types[p]\nsource_path=tostring(null)\ncontent=local.template_file_contents[p]\ndigests=tomap({\nmd5=md5(local.template_file_contents[p])\nsha1=sha1(local.template_file_contents[p])\nsha256=sha256(local.template_file_contents[p])\nsha512=sha512(local.template_file_contents[p])\nbase64sha256=base64sha256(local.template_file_contents[p])\nbase64sha512=base64sha512(local.template_file_contents[p])\n})\n}\n},\n{\nforpinlocal.static_file_paths:p=>{\ncontent_type=local.file_types[p]\nsource_path=local.static_file_local_paths[p]\ncontent=tostring(null)\ndigests=tomap({\nmd5=filemd5(local.static_file_local_paths[p])\nsha1=filesha1(local.static_file_local_paths[p])\nsha256=filesha256(local.static_file_local_paths[p])\nsha512=filesha512(local.static_file_local_paths[p])\nbase64sha256=filebase64sha256(local.static_file_local_paths[p])\nbase64sha512=filebase64sha512(local.static_file_local_paths[p])\n})\n}\n},\n)")
