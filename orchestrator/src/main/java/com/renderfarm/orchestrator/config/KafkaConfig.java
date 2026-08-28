package com.renderfarm.orchestrator.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

@Configuration
public class KafkaConfig {

    @Value("${render-farm.topics.jobs:render.jobs}")
    private String jobsTopic;

    @Value("${render-farm.topics.results:render.results}")
    private String resultsTopic;

    @Bean
    public NewTopic renderJobsTopic() {
        return TopicBuilder.name(jobsTopic)
                .partitions(8)
                .replicas(1)
                .build();
    }

    @Bean
    public NewTopic renderResultsTopic() {
        return TopicBuilder.name(resultsTopic)
                .partitions(8)
                .replicas(1)
                .build();
    }
}
