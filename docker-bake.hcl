variable "GO_IMAGE" {
  default = "golang:1.24.9-bookworm"
}

variable "CUE_VERSION" {
  default = "v0.15.4"
}

target "_docker_verify" {
  context = "."
  dockerfile = "Dockerfile.test"
  target = "verify"
  output = ["type=cacheonly"]
  platforms = ["linux/amd64"]
}

target "docker-check" {
  inherits = ["_docker_verify"]
  args = {
    GO_IMAGE = "golang:1.24.9-bookworm"
    CUE_VERSION = CUE_VERSION
  }
}

target "docker-check-trixie" {
  inherits = ["_docker_verify"]
  args = {
    GO_IMAGE = "golang:1.24.9-trixie"
    CUE_VERSION = CUE_VERSION
  }
}

group "docker-matrix" {
  targets = ["docker-check", "docker-check-trixie"]
}
