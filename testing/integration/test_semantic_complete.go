package main

import "fmt"

// @lci:labels[api,checkout,critical]
// @lci:category[payment-endpoint]
// @lci:deps[database:orders:write,service:payment:read-write]
// @lci:metrics[avg_duration=250ms,complexity=high]
// @lci:propagate[attr=checkout,dir=downstream,decay=0.8,hops=5]
func HandleCheckout(orderID string) error {
	return ProcessPayment(orderID)
}

// @lci:labels[payment,security,high-priority]
// @lci:category[payment-processor]
// @lci:deps[service:stripe:read-write,database:transactions:write]
// @lci:metrics[avg_duration=180ms,complexity=medium]
func ProcessPayment(orderID string) error {
	return ValidatePayment(orderID)
}

// @lci:labels[security,validation]
// @lci:category[validation]
// @lci:deps[service:auth:read,database:users:read]
func ValidatePayment(orderID string) error {
	fmt.Printf("Validating payment for order: %s\n", orderID)
	return nil
}

func main() {
	_ = HandleCheckout("order-123")
}
