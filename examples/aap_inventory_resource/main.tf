terraform {
  required_providers {
    aap = {
      source = "registry.terraform.io/ansible/aap"
    }
  }
}

provider "aap" {
  host     = "https://localhost:8043"
  username = "ansible"
  password = "test123!"
  insecure_skip_verify = true
}

resource "aap_aap_inventory" "my_inventory" {
  name = "My new inventory"
  description = "A new inventory for testing"
  groups = [
    {
      name = "My new group"
      children = ["Group 2"]
      description = "A new group for testing"
      variables = {
        groupvar1 = "foo"
        groupvar2 = "bar"
      }
    },
    {
    name = "Group 2"
    description = "A second new group for testing"
    variables = {
      groupvar1 = "foo"
      groupvar2 = "bar"
    }
  }
  ]
  hosts = [
    {
      name = "My new host"
      description = "A new host for testing"
      groups = [ "My new group", "Group 2" ]
      variables = {
        hostvar1 = "foo"
        hostvar2 = "bar"
      }
    }
  ]
  variables = {
    inventoryvar1 = "foo"
    inventoryvar2 = "bar"
  }
}

output "inventory" {
  value = aap_aap_inventory.my_inventory
}
