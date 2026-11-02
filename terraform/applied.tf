# The trunk commit this project was last applied from (#2680). CI's drift
# check plans this commit, not trunk's head, so a merged change still waiting
# for its apply is not drift. No default: an apply without the stamp refuses.
variable "applied_sha" {
  type        = string
  description = "Trunk's newest commit that touched terraform/ (outside terraform/stripe*): git log -1 --format=%H -- terraform ':(exclude)terraform/stripe*'"

  validation {
    condition     = can(regex("^[0-9a-f]{40}$", var.applied_sha))
    error_message = "applied_sha must be a full 40-character commit SHA."
  }
}

resource "terraform_data" "applied" {
  input = var.applied_sha
}

output "applied_sha" {
  value = terraform_data.applied.output
}
