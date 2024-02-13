provider "aws" {
  region = var.region
}

data "aws_regions"  "available" {
  all_regions = true
}

output "available_regions" {
  value = join(",", data.aws_regions.available.names)
}

variable "region" {
  description = "AWS region"
  type        = string
}

