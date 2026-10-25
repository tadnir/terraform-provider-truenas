# Run a cron job now (Terraform 1.14+) and wait for its command to finish.
# id is the numeric job id — reference the managing resource rather than
# hardcoding it (or discover it with `midclt call cronjob.query`).
action "truenas_cronjob_run" "example" {
  config {
    id = truenas_cronjob.cleanup.id
  }
}

# A disabled cron job is never scheduled, but still runs on demand. With an
# action_trigger, its command runs once each time Terraform creates it.
resource "truenas_cronjob" "fetch" {
  description = "Download a file"
  user        = "root"
  command     = "curl -fsSL -o /mnt/tank/files/data.bin https://example.com/data.bin"
  enabled     = false

  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "1"
    month  = "1"
    dow    = "*"
  }

  lifecycle {
    action_trigger {
      events  = [after_create, after_update]
      actions = [action.truenas_cronjob_run.fetch]
    }
  }
}

# caller is the resource whose action_trigger runs it.
action "truenas_cronjob_run" "fetch" {
  config {
    id = caller.id
  }
}
