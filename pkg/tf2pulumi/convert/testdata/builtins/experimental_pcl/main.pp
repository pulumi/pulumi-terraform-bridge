# A load of the examples in the docs use `path.module` which _should_ resolve to the file system path of
# the current module, but tf2pulumi doesn't support that so we replace it with local.path_module.
pathModule = "some/path"
# Some of the examples in the docs use `path.root` which _should_ resolve to the file system path of the
# root module of the configuration, but tf2pulumi doesn't support that so we replace it with
pathRoot = "root/path"
# Examples for abs
output "funcAbs0" {
  value = invoke("std:index:abs", {
    input = 23
  }).result
}
output "funcAbs1" {
  value = invoke("std:index:abs", {
    input = 0
  }).result
}
output "funcAbs2" {
  value = invoke("std:index:abs", {
    input = -12.4
  }).result
}
# Examples for abspath
output "funcAbspath" {
  value = invoke("std:index:abspath", {
    input = pathRoot
  }).result
}
# Examples for base64decode
output "funcBase64decode" {
  value = invoke("std:index:base64decode", {
    input = "SGVsbG8gV29ybGQ="
  }).result
}
# Examples for base64encode
output "funcBase64encode" {
  value = invoke("std:index:base64encode", {
    input = "Hello World"
  }).result
}
# Examples for base64gzip
output "funcBase64gzip" {
  value = invoke("std:index:base64gzip", {
    input = "test"
  }).result
}
# Examples for base64sha256
output "funcBase64sha256" {
  value = invoke("std:index:base64sha256", {
    input = "hello world"
  }).result
}
# Examples for base64sha512
output "funcBase64sha512" {
  value = invoke("std:index:base64sha512", {
    input = "hello world"
  }).result
}
# Examples for basename
output "funcBasename" {
  value = invoke("std:index:basename", {
    input = "foo/bar/baz.txt"
  }).result
}
# Examples for bcrypt
output "funcBcrypt" {
  value = invoke("std:index:bcrypt", {
    input = "hello world"
  }).result
}
# Examples for ceil
output "funcCeil0" {
  value = invoke("std:index:ceil", {
    input = 5
  }).result
}
output "funcCeil1" {
  value = invoke("std:index:ceil", {
    input = 5.1
  }).result
}
# Examples for chomp
output "funcChomp0" {
  value = invoke("std:index:chomp", {
    input = "hello\n"
  }).result
}
output "funcChomp1" {
  value = invoke("std:index:chomp", {
    input = "hello\r\n"
  }).result
}
output "funcChomp2" {
  value = invoke("std:index:chomp", {
    input = "hello\n\n"
  }).result
}
# Examples for cidrhost
output "funcCidrhost0" {
  value = invoke("std:index:cidrhost", {
    input = "10.12.112.0/20"
    host  = 16
  }).result
}
output "funcCidrhost1" {
  value = invoke("std:index:cidrhost", {
    input = "10.12.112.0/20"
    host  = 268
  }).result
}
output "funcCidrhost2" {
  value = invoke("std:index:cidrhost", {
    input = "fd00:fd12:3456:7890:00a2::/72"
    host  = 34
  }).result
}
# Examples for cidrnetmask
output "funcCidrnetmask" {
  value = invoke("std:index:cidrnetmask", {
    input = "172.16.0.0/12"
  }).result
}
# Examples for cidrsubnet
output "funcCidrsubnet0" {
  value = invoke("std:index:cidrsubnet", {
    input   = "172.16.0.0/12"
    newbits = 4
    netnum  = 2
  }).result
}
output "funcCidrsubnet1" {
  value = invoke("std:index:cidrsubnet", {
    input   = "10.1.2.0/24"
    newbits = 4
    netnum  = 15
  }).result
}
output "funcCidrsubnet2" {
  value = invoke("std:index:cidrsubnet", {
    input   = "fd00:fd12:3456:7890::/56"
    newbits = 16
    netnum  = 162
  }).result
}
output "funcCidrsubnet3" {
  value = invoke("std:index:cidrhost", {
    input = "10.1.2.240/28"
    host  = 1
  }).result
}
output "funcCidrsubnet4" {
  value = invoke("std:index:cidrhost", {
    input = "10.1.2.240/28"
    host  = 14
  }).result
}
# Examples for compact
output "funcCompact" {
  value = invoke("std:index:compact", {
    input = ["a", "", "b", "c"]
  }).result
}
# Examples for csvdecode
output "funcCsvdecode" {
  value = invoke("std:index:csvdecode", {
    input = "a,b,c\n1,2,3\n4,5,6"
  }).result
}
# Examples for dirname
output "funcDirname" {
  value = invoke("std:index:dirname", {
    input = "foo/bar/baz.txt"
  }).result
}
# Examples for element
output "funcElement0" {
  value = element(["a", "b", "c"], 1)
}
output "funcElement1" {
  value = element(["a", "b", "c"], 3)
}
output "funcElement2" {
  value = element(["a", "b", "c"], length(["a", "b", "c"]) - 1)
}
# Examples for endswith
output "funcEndswith0" {
  value = invoke("std:index:endswith", {
    input  = "hello world"
    suffix = "world"
  }).result
}
output "funcEndswith1" {
  value = invoke("std:index:endswith", {
    input  = "hello world"
    suffix = "hello"
  }).result
}
# Examples for file
output "funcFile" {
  value = invoke("std:index:file", {
    input = "${pathModule}/hello.txt"
  }).result
}
# Examples for filebase64
output "funcFilebase64" {
  value = invoke("std:index:filebase64", {
    input = "${pathModule}/hello.txt"
  }).result
}
# Examples for filebase64sha256
output "funcFilebase64sha256" {
  value = invoke("std:index:filebase64sha256", {
    input = "hello.txt"
  }).result
}
# Examples for filebase64sha512
output "funcFilebase64sha512" {
  value = invoke("std:index:filebase64sha512", {
    input = "hello.txt"
  }).result
}
# Examples for fileexists
output "funcFileexists" {
  value = invoke("std:index:fileexists", {
    input = "${pathModule}/hello.txt"
  }).result
}
# Examples for filemd5
output "funcFilemd5" {
  value = invoke("std:index:filemd5", {
    input = "hello.txt"
  }).result
}
# Examples for filesha1
output "funcFilesha1" {
  value = invoke("std:index:filesha1", {
    input = "hello.txt"
  }).result
}
# Examples for filesha256
output "funcFilesha256" {
  value = invoke("std:index:filesha256", {
    input = "hello.txt"
  }).result
}
# Examples for filesha512
output "funcFilesha512" {
  value = invoke("std:index:filesha512", {
    input = "hello.txt"
  }).result
}
# Examples for floor
output "funcFloor0" {
  value = invoke("std:index:floor", {
    input = 5
  }).result
}
output "funcFloor1" {
  value = invoke("std:index:floor", {
    input = 4.9
  }).result
}
# Examples for indent
output "funcIndent" {
  value = "  items: ${invoke("std:index:indent", {
    spaces = 2
    input  = "[\n  foo,\n  bar,\n]\n"
  }).result}"
}
# Examples for join
output "funcJoin0" {
  value = invoke("std:index:join", {
    separator = ", "
    input     = ["foo", "bar", "baz"]
  }).result
}
output "funcJoin1" {
  value = invoke("std:index:join", {
    separator = ", "
    input     = ["foo"]
  }).result
}
# Examples for jsonencode
output "funcJsonencode" {
  value = toJSON({
    "hello" = "world"
  })
}
# Examples for length
output "funcLength0" {
  value = length([])
}
output "funcLength1" {
  value = length(["a", "b"])
}
output "funcLength2" {
  value = length({
    "a" = "b"
  })
}
output "funcLength3" {
  value = length("hello")
}
output "funcLength4" {
  value = length("👾🕹️")
}
# Examples for log
output "funcLog0" {
  value = invoke("std:index:log", {
    base  = 50
    input = 10
  }).result
}
output "funcLog1" {
  value = invoke("std:index:log", {
    base  = 16
    input = 2
  }).result
}
output "funcLog2" {
  value = invoke("std:index:ceil", {
    input = invoke("std:index:log", {
      base  = 15
      input = 2
    }).result
  }).result
}
output "funcLog3" {
  value = invoke("std:index:ceil", {
    input = invoke("std:index:log", {
      base  = 16
      input = 2
    }).result
  }).result
}
output "funcLog4" {
  value = invoke("std:index:ceil", {
    input = invoke("std:index:log", {
      base  = 17
      input = 2
    }).result
  }).result
}
# Examples for lookup
output "funcLookup0" {
  value = lookup({
    a = "ay"
    b = "bee"
  }, "a", "what?")
}
output "funcLookup1" {
  value = lookup({
    a = "ay"
    b = "bee"
  }, "c", "what?")
}
# Examples for lower
output "funcLower0" {
  value = invoke("std:index:lower", {
    input = "HELLO"
  }).result
}
output "funcLower1" {
  value = invoke("std:index:lower", {
    input = "АЛЛО!"
  }).result
}
# Examples for max
output "funcMax0" {
  value = invoke("std:index:max", {
    input = [12, 54, 3]
  }).result
}
output "funcMax1" {
  value = invoke("std:index:max", {
    input = [12, 54, 3]
  }).result
}
# Examples for md5
output "funcMd5" {
  value = invoke("std:index:md5", {
    input = "hello world"
  }).result
}
# Examples for min
output "funcMin0" {
  value = invoke("std:index:min", {
    input = [12, 54, 3]
  }).result
}
output "funcMin1" {
  value = invoke("std:index:min", {
    input = [12, 54, 3]
  }).result
}
# Examples for parseint
output "funcParseint0" {
  value = invoke("std:index:parseint", {
    base  = "100"
    input = 10
  }).result
}
output "funcParseint1" {
  value = invoke("std:index:parseint", {
    base  = "FF"
    input = 16
  }).result
}
output "funcParseint2" {
  value = invoke("std:index:parseint", {
    base  = "-10"
    input = 16
  }).result
}
output "funcParseint3" {
  value = invoke("std:index:parseint", {
    base  = "1011111011101111"
    input = 2
  }).result
}
output "funcParseint4" {
  value = invoke("std:index:parseint", {
    base  = "aA"
    input = 62
  }).result
}
output "funcParseint5" {
  value = invoke("std:index:parseint", {
    base  = "12"
    input = 2
  }).result
}
# Examples for pathexpand
output "funcPathexpand0" {
  value = invoke("std:index:pathexpand", {
    input = "~/.ssh/id_rsa"
  }).result
}
output "funcPathexpand1" {
  value = invoke("std:index:pathexpand", {
    input = "/etc/resolv.conf"
  }).result
}
# Examples for pow
output "funcPow0" {
  value = invoke("std:index:pow", {
    base     = 3
    exponent = 2
  }).result
}
output "funcPow1" {
  value = invoke("std:index:pow", {
    base     = 4
    exponent = 0
  }).result
}
# Examples for range
output "funcRange0" {
  value = invoke("std:index:range", {
    limit = 3
  }).result
}
output "funcRange1" {
  value = invoke("std:index:range", {
    limit = 1
    start = 4
  }).result
}
output "funcRange2" {
  value = invoke("std:index:range", {
    limit = 1
    start = 8
    step  = 2
  }).result
}
output "funcRange3" {
  value = invoke("std:index:range", {
    limit = 1
    start = 4
    step  = 0.5
  }).result
}
output "funcRange4" {
  value = invoke("std:index:range", {
    limit = 4
    start = 1
  }).result
}
output "funcRange5" {
  value = invoke("std:index:range", {
    limit = 10
    start = 5
    step  = -2
  }).result
}
# Examples for replace
output "funcReplace0" {
  value = invoke("std:index:replace", {
    replace = "1 + 2 + 3"
    search  = "+"
    text    = "-"
  }).result
}
output "funcReplace1" {
  value = invoke("std:index:replace", {
    replace = "hello world"
    search  = "/w.*d/"
    text    = "everybody"
  }).result
}
# Examples for rsadecrypt
output "funcRsadecrypt" {
  value = invoke("std:index:rsadecrypt", {
    cipherText = invoke("std:index:filebase64", {
      input = "${pathModule}/ciphertext"
    }).result
    key = invoke("std:index:file", {
      input = "privatekey.pem"
    }).result
  }).result
}
# Examples for sensitive
output "funcSensitive0" {
  value = secret(1)
}
output "funcSensitive1" {
  value = secret("hello")
}
output "funcSensitive2" {
  value = secret([])
}
# Examples for sha1
output "funcSha1" {
  value = invoke("std:index:sha1", {
    input = "hello world"
  }).result
}
# Examples for sha256
output "funcSha256" {
  value = invoke("std:index:sha256", {
    input = "hello world"
  }).result
}
# Examples for sha512
output "funcSha512" {
  value = invoke("std:index:sha512", {
    input = "hello world"
  }).result
}
# Examples for signum
output "funcSignum0" {
  value = invoke("std:index:signum", {
    input = -13
  }).result
}
output "funcSignum1" {
  value = invoke("std:index:signum", {
    input = 0
  }).result
}
output "funcSignum2" {
  value = invoke("std:index:signum", {
    input = 344
  }).result
}
# Examples for sort
output "funcSort" {
  value = invoke("std:index:sort", {
    input = ["e", "d", "a", "x"]
  }).result
}
# Examples for split
output "funcSplit0" {
  value = invoke("std:index:split", {
    separator = ","
    text      = "foo,bar,baz"
  }).result
}
output "funcSplit1" {
  value = invoke("std:index:split", {
    separator = ","
    text      = "foo"
  }).result
}
output "funcSplit2" {
  value = invoke("std:index:split", {
    separator = ","
    text      = ""
  }).result
}
# Examples for startswith
output "funcStartswith0" {
  value = invoke("std:index:startswith", {
    input  = "hello world"
    prefix = "hello"
  }).result
}
output "funcStartswith1" {
  value = invoke("std:index:startswith", {
    input  = "hello world"
    prefix = "world"
  }).result
}
# Examples for strrev
output "funcStrrev0" {
  value = invoke("std:index:strrev", {
    input = "hello"
  }).result
}
output "funcStrrev1" {
  value = invoke("std:index:strrev", {
    input = "a ☃"
  }).result
}
# Examples for substr
output "funcSubstr0" {
  value = invoke("std:index:substr", {
    input  = "hello world"
    length = 1
    offset = 4
  }).result
}
output "funcSubstr1" {
  value = invoke("std:index:substr", {
    input  = "🤔🤷"
    length = 0
    offset = 1
  }).result
}
output "funcSubstr2" {
  value = invoke("std:index:substr", {
    input  = "hello world"
    length = -5
    offset = -1
  }).result
}
output "funcSubstr3" {
  value = invoke("std:index:substr", {
    input  = "hello world"
    length = 6
    offset = 10
  }).result
}
# Examples for sum
output "funcSum" {
  value = invoke("std:index:sum", {
    input = [10, 13, 6, 4.5]
  }).result
}
# Examples for timeadd
output "funcTimeadd" {
  value = invoke("std:index:timeadd", {
    duration  = "2017-11-22T00:00:00Z"
    timestamp = "10m"
  }).result
}
# Examples for timecmp
output "funcTimecmp0" {
  value = invoke("std:index:timecmp", {
    timestampa = "2017-11-22T00:00:00Z"
    timestampb = "2017-11-22T00:00:00Z"
  }).result
}
output "funcTimecmp1" {
  value = invoke("std:index:timecmp", {
    timestampa = "2017-11-22T00:00:00Z"
    timestampb = "2017-11-22T01:00:00Z"
  }).result
}
output "funcTimecmp2" {
  value = invoke("std:index:timecmp", {
    timestampa = "2017-11-22T01:00:00Z"
    timestampb = "2017-11-22T00:00:00Z"
  }).result
}
output "funcTimecmp3" {
  value = invoke("std:index:timecmp", {
    timestampa = "2017-11-22T01:00:00Z"
    timestampb = "2017-11-22T00:00:00-01:00"
  }).result
}
# Examples for timestamp
output "funcTimestamp" {
  value = invoke("std:index:timestamp", {}).result
}
# Examples for title
output "funcTitle" {
  value = invoke("std:index:title", {
    input = "hello world"
  }).result
}
# Examples for transpose
output "funcTranspose" {
  value = invoke("std:index:transpose", {
    input = {
      "a" = ["1", "2"]
      "b" = ["2", "3"]
    }
  }).result
}
# Examples for trim
output "funcTrim0" {
  value = invoke("std:index:trim", {
    input  = "?!hello?!"
    cutset = "!?"
  }).result
}
output "funcTrim1" {
  value = invoke("std:index:trim", {
    input  = "foobar"
    cutset = "far"
  }).result
}
output "funcTrim2" {
  value = invoke("std:index:trim", {
    input  = "   hello! world.!  "
    cutset = "! "
  }).result
}
# Examples for trimprefix
output "funcTrimprefix0" {
  value = invoke("std:index:trimprefix", {
    input  = "helloworld"
    prefix = "hello"
  }).result
}
output "funcTrimprefix1" {
  value = invoke("std:index:trimprefix", {
    input  = "helloworld"
    prefix = "cat"
  }).result
}
# Examples for trimspace
output "funcTrimspace" {
  value = invoke("std:index:trimspace", {
    input = "  hello\n\n"
  }).result
}
# Examples for trimsuffix
output "funcTrimsuffix" {
  value = invoke("std:index:trimsuffix", {
    input  = "helloworld"
    suffix = "world"
  }).result
}
# Examples for upper
output "funcUpper0" {
  value = invoke("std:index:upper", {
    input = "hello"
  }).result
}
output "funcUpper1" {
  value = invoke("std:index:upper", {
    input = "алло!"
  }).result
}
# Examples for urlencode
output "funcUrlencode0" {
  value = invoke("std:index:urlencode", {
    input = "Hello World!"
  }).result
}
output "funcUrlencode1" {
  value = invoke("std:index:urlencode", {
    input = "☃"
  }).result
}
output "funcUrlencode2" {
  value = "http://example.com/search?q=${invoke("std:index:urlencode", {
    input = "terraform urlencode"
  }).result}"
}
# Examples for uuid
output "funcUuid" {
  value = invoke("std:index:uuid", {}).result
}
