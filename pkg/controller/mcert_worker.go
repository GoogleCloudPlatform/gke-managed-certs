package controller

import (
)

func (c *Controller) runMcertWorker() {
	for c.processNextMcert() {
	}
}

func (c *Controller) processNextMcert() bool {
	return true
}
