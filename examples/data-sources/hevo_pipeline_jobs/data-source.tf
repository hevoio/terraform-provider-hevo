data "hevo_pipeline_jobs" "recent" {
  pipeline_id = hevo_pipeline.postgres.id
  limit       = 5
}

output "last_job_status" {
  value = try(data.hevo_pipeline_jobs.recent.jobs[0].status, "no jobs yet")
}
