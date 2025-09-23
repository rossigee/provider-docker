/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package compose

import (
	"context"
	"testing"
)

func TestParser_ParseCompose(t *testing.T) {
	tests := []struct {
		name           string
		projectName    string
		workingDir     string
		environment    map[string]string
		composeContent string
		wantErr        bool
		wantContainers int
		validateResult func(t *testing.T, result *ParseResult)
	}{
		{
			name:        "simple service",
			projectName: "test-project",
			workingDir:  "",
			environment: nil,
			composeContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
`,
			wantErr:        false,
			wantContainers: 1,
			validateResult: func(t *testing.T, result *ParseResult) {
				if len(result.Containers) != 1 {
					t.Errorf("Expected 1 container, got %d", len(result.Containers))
					return
				}

				container := result.Containers[0]
				if container.Spec.ForProvider.Image != "nginx:latest" {
					t.Errorf("Expected image nginx:latest, got %s", container.Spec.ForProvider.Image)
				}

				if len(container.Spec.ForProvider.Ports) != 1 {
					t.Errorf("Expected 1 port, got %d", len(container.Spec.ForProvider.Ports))
					return
				}

				port := container.Spec.ForProvider.Ports[0]
				if port.ContainerPort != 80 {
					t.Errorf("Expected container port 80, got %d", port.ContainerPort)
				}
				if port.HostPort == nil || *port.HostPort != 80 {
					t.Errorf("Expected host port 80, got %v", port.HostPort)
				}
			},
		},
		{
			name:        "service with environment variables",
			projectName: "test-env",
			workingDir:  "",
			environment: map[string]string{
				"NODE_ENV": "production",
				"PORT":     "3000",
			},
			composeContent: `
version: '3.8'
services:
  api:
    image: node:18-alpine
    environment:
      NODE_ENV: ${NODE_ENV}
      PORT: ${PORT}
      DEBUG: "true"
    ports:
      - "${PORT}:${PORT}"
`,
			wantErr:        false,
			wantContainers: 1,
			validateResult: func(t *testing.T, result *ParseResult) {
				if len(result.Containers) != 1 {
					t.Errorf("Expected 1 container, got %d", len(result.Containers))
					return
				}

				container := result.Containers[0]

				// Check that we have the right number of environment variables
				if len(container.Spec.ForProvider.Environment) != 3 {
					t.Errorf("Expected 3 environment variables, got %d", len(container.Spec.ForProvider.Environment))
				}

				// Check that key environment variables are present
				envMap := make(map[string]*string)
				for _, env := range container.Spec.ForProvider.Environment {
					envMap[env.Name] = env.Value
				}

				// Verify NODE_ENV is set correctly (should be interpolated)
				if nodeEnv, exists := envMap["NODE_ENV"]; !exists {
					t.Errorf("Expected NODE_ENV environment variable not found")
				} else if nodeEnv == nil || *nodeEnv != "production" {
					t.Errorf("Expected NODE_ENV=production, got %v", nodeEnv)
				}
			},
		},
		{
			name:        "service with volumes",
			projectName: "test-volumes",
			workingDir:  "",
			environment: nil,
			composeContent: `
version: '3.8'
services:
  db:
    image: postgres:13
    volumes:
      - db_data:/var/lib/postgresql/data
      - ./config:/etc/postgresql/conf.d:ro
    environment:
      POSTGRES_DB: myapp

volumes:
  db_data:
`,
			wantErr:        false,
			wantContainers: 1,
			validateResult: func(t *testing.T, result *ParseResult) {
				if len(result.Containers) != 1 {
					t.Errorf("Expected 1 container, got %d", len(result.Containers))
					return
				}

				container := result.Containers[0]
				if len(container.Spec.ForProvider.Volumes) != 2 {
					t.Errorf("Expected 2 volumes, got %d", len(container.Spec.ForProvider.Volumes))
					return
				}

				// Check named volume
				namedVolume := container.Spec.ForProvider.Volumes[0]
				if namedVolume.Name != "db_data" {
					t.Errorf("Expected volume name db_data, got %s", namedVolume.Name)
				}
				if namedVolume.MountPath != "/var/lib/postgresql/data" {
					t.Errorf("Expected mount path /var/lib/postgresql/data, got %s", namedVolume.MountPath)
				}

				// Check bind mount
				bindMount := container.Spec.ForProvider.Volumes[1]
				if bindMount.MountPath != "/etc/postgresql/conf.d" {
					t.Errorf("Expected mount path /etc/postgresql/conf.d, got %s", bindMount.MountPath)
				}
				if bindMount.ReadOnly == nil || !*bindMount.ReadOnly {
					t.Errorf("Expected read-only mount")
				}

				// Check volumes definition
				if len(result.Volumes) != 1 {
					t.Errorf("Expected 1 volume definition, got %d", len(result.Volumes))
					return
				}

				volumeDef := result.Volumes[0]
				if volumeDef.Name != "test-volumes_db_data" {
					t.Errorf("Expected volume name test-volumes_db_data, got %s", volumeDef.Name)
				}
			},
		},
		{
			name:        "multi-service with dependencies",
			projectName: "test-deps",
			workingDir:  "",
			environment: nil,
			composeContent: `
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  api:
    image: node:18-alpine
    ports:
      - "3000:3000"
    depends_on:
      - redis
    environment:
      REDIS_URL: redis://redis:6379

  web:
    image: nginx:latest
    ports:
      - "80:80"
    depends_on:
      - api
`,
			wantErr:        false,
			wantContainers: 3,
			validateResult: func(t *testing.T, result *ParseResult) {
				if len(result.Containers) != 3 {
					t.Errorf("Expected 3 containers, got %d", len(result.Containers))
					return
				}

				// Check that containers are present
				containerNames := make(map[string]bool)
				for _, container := range result.Containers {
					if container.Spec.ForProvider.Name != nil {
						containerNames[*container.Spec.ForProvider.Name] = true
					}
				}

				expectedNames := []string{"redis", "api", "web"}
				for _, name := range expectedNames {
					if !containerNames[name] {
						t.Errorf("Expected container %s not found", name)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.projectName, tt.workingDir, tt.environment)

			result, err := parser.ParseCompose(context.Background(), tt.composeContent)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCompose() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Skip validation if error was expected
			}

			if len(result.Containers) != tt.wantContainers {
				t.Errorf("Expected %d containers, got %d", tt.wantContainers, len(result.Containers))
			}

			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestParser_GetServiceDependencies(t *testing.T) {
	tests := []struct {
		name           string
		composeContent string
		wantDeps       map[string][]string
	}{
		{
			name: "simple dependencies",
			composeContent: `
version: '3.8'
services:
  redis:
    image: redis:7-alpine

  api:
    image: node:18-alpine
    depends_on:
      - redis

  web:
    image: nginx:latest
    depends_on:
      - api
`,
			wantDeps: map[string][]string{
				"api": {"redis"},
				"web": {"api"},
			},
		},
		{
			name: "multiple dependencies",
			composeContent: `
version: '3.8'
services:
  redis:
    image: redis:7-alpine

  postgres:
    image: postgres:13

  api:
    image: node:18-alpine
    depends_on:
      - redis
      - postgres

  web:
    image: nginx:latest
    depends_on:
      - api
`,
			wantDeps: map[string][]string{
				"api": {"redis", "postgres"}, // Order doesn't matter with new test logic
				"web": {"api"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser("test", "", nil)

			result, err := parser.ParseCompose(context.Background(), tt.composeContent)
			if err != nil {
				t.Fatalf("ParseCompose() error = %v", err)
			}

			deps := parser.GetServiceDependencies(result.Project)

			// Compare dependencies with order-insensitive comparison
			for service, expectedDeps := range tt.wantDeps {
				actualDeps, exists := deps[service]
				if !exists {
					t.Errorf("Expected service %s not found in dependencies", service)
					continue
				}

				if len(actualDeps) != len(expectedDeps) {
					t.Errorf("Service %s: expected %d dependencies, got %d", service, len(expectedDeps), len(actualDeps))
					continue
				}

				// Check that all expected dependencies are present (order doesn't matter)
				expectedSet := make(map[string]bool)
				for _, dep := range expectedDeps {
					expectedSet[dep] = true
				}

				for _, dep := range actualDeps {
					if !expectedSet[dep] {
						t.Errorf("Service %s: unexpected dependency %s", service, dep)
					}
				}

				actualSet := make(map[string]bool)
				for _, dep := range actualDeps {
					actualSet[dep] = true
				}

				for _, dep := range expectedDeps {
					if !actualSet[dep] {
						t.Errorf("Service %s: missing expected dependency %s", service, dep)
					}
				}
			}

			// Check for unexpected services
			for service := range deps {
				if _, expected := tt.wantDeps[service]; !expected {
					t.Errorf("Unexpected service %s found in dependencies", service)
				}
			}
		})
	}
}

func TestParser_ValidateCompose(t *testing.T) {
	tests := []struct {
		name           string
		composeContent string
		wantErr        bool
	}{
		{
			name: "valid compose file",
			composeContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
`,
			wantErr: false,
		},
		{
			name: "invalid yaml",
			composeContent: `
version: '3.8'
services:
  web:
    image: nginx:latest
    invalid_yaml: [
`,
			wantErr: true,
		},
		{
			name: "missing required fields",
			composeContent: `
version: '3.8'
services:
  web:
    # missing image
    ports:
      - "80:80"
`,
			wantErr: true, // compose-go should require image or build
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser("test", "", nil)

			err := parser.ValidateCompose(context.Background(), tt.composeContent)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCompose() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
