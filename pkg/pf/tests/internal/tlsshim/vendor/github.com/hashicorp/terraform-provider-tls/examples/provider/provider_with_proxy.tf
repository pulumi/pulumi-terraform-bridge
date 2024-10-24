# This example fetches the TLS certificate chain
# from `example.com` using an HTTP Proxy.

provider "tls" {
  proxy {
    url = "https://corporate.proxy.service"
  }
}

data "tls_certificate" "test" {
  url = "https://example.com"
}
