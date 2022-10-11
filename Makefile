# Copyright 2015 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_ARCHS ?= amd64 armv7 arm64 ppc64le s390x
DOCKER_IMAGE_NAME ?= haproxy-exporter

all:: vet checkmetrics common-all

include Makefile.common

PROMETHEUS_VERSION=2.39.1
PROMTOOL ?= /tmp/prometheus-$(PROMETHEUS_VERSION).linux-amd64/promtool

.PHONY: checkmetrics
checkmetrics:
	@echo ">> checking metrics for correctness"
	if ! test -x $(PROMTOOL); then curl -sL -o - https://github.com/prometheus/prometheus/releases/download/v$(PROMETHEUS_VERSION)/prometheus-$(PROMETHEUS_VERSION).linux-amd64.tar.gz | tar -C /tmp -xzf - prometheus-$(PROMETHEUS_VERSION).linux-amd64/promtool; fi
	for file in test/*.metrics; do $(PROMTOOL) check metrics < $$file || exit 1; done
