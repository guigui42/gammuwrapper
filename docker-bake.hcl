
variable "ECR_REGISTRY" {
  default = "guigui4242"
}

variable "TAGVERSION" {
  default = formatdate("YYYY.MM.DD", timestamp())
}

variable "TAGGIT" {
  default = "nogit"
}

target "default" {

  context    = "."
  dockerfile = "Dockerfile"
  tags = [
    "${ECR_REGISTRY}/gammuwrapper:latest",
    "${ECR_REGISTRY}/gammuwrapper:${TAGVERSION}",
    "${ECR_REGISTRY}/gammuwrapper:${TAGVERSION}-${TAGGIT}",
  ]
  labels = {
    builton = timestamp()
  }

}
