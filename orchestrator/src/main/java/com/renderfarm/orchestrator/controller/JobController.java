package com.renderfarm.orchestrator.controller;

import com.renderfarm.orchestrator.model.JobRequest;
import com.renderfarm.orchestrator.model.JobStatusResponse;
import com.renderfarm.orchestrator.service.JobOrchestratorService;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/v1")
@CrossOrigin(origins = "*")
public class JobController {

    private final JobOrchestratorService orchestratorService;

    public JobController(JobOrchestratorService orchestratorService) {
        this.orchestratorService = orchestratorService;
    }

    @GetMapping("/health")
    public ResponseEntity<Map<String, String>> health() {
        return ResponseEntity.ok(Map.of("status", "UP", "service", "render-farm-orchestrator"));
    }

    @PostMapping("/jobs")
    public ResponseEntity<JobStatusResponse> submitJob(@RequestBody JobRequest request) {
        if (request.getSourcePath() == null || request.getSourcePath().isBlank()) {
            return ResponseEntity.badRequest().build();
        }
        JobStatusResponse response = orchestratorService.submitJob(request);
        return ResponseEntity.status(HttpStatus.ACCEPTED).body(response);
    }

    @GetMapping("/jobs/{jobId}")
    public ResponseEntity<JobStatusResponse> getJobStatus(@PathVariable String jobId) {
        JobStatusResponse status = orchestratorService.getJobStatus(jobId);
        if (status == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(status);
    }

    @GetMapping("/jobs/{jobId}/result")
    public ResponseEntity<?> getJobResult(@PathVariable String jobId) {
        JobStatusResponse status = orchestratorService.getJobStatus(jobId);
        if (status == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok(Map.of(
                "jobId", status.getJobId(),
                "status", status.getStatus(),
                "finalOutputPath", status.getFinalOutputPath() != null ? status.getFinalOutputPath() : "",
                "totalDurationMs", status.getTotalDurationMs() != null ? status.getTotalDurationMs() : 0,
                "progressPercent", status.getProgressPercent()
        ));
    }
}
