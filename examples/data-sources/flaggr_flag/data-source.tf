data "flaggr_flag" "checkout" {
  key         = "new-checkout-flow"
  service_id  = data.flaggr_service.api.id
  environment = "production"
}

output "checkout_enabled" {
  value = data.flaggr_flag.checkout.enabled
}
