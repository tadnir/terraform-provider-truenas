# Run a cron job immediately (Terraform 1.14+). A disabled cron job (one that
# is never scheduled) is a handy way to have the box run a one-off command as
# part of an apply — e.g. fetch a file before another resource uses it.
resource "truenas_cronjob" "fetch" {
  description = "Download a file"
  user        = "root"
  command     = "curl -fsSL -o /mnt/tank/files/data.bin https://example.com/data.bin"
  enabled     = false # never scheduled; run only by the action below

  schedule = { minute = "0", hour = "0", dom = "1", month = "1", dow = "*" }

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.truenas_cronjob_run.fetch]
    }
  }
}

# Waits for the job and fails if the command exits non-zero. Reference the
# triggering resource with caller.id inside an action_trigger.
action "truenas_cronjob_run" "fetch" {
  config {
    id = caller.id
    # skip_disabled = false  # a disabled job still runs unless this is true
  }
}
