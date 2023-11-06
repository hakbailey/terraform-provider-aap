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
      name = "Group A"
      children = ["Group B", "Group C"]
      description = "A new group for testing"
      variables = {
        groupvar1 = "foo"
        groupvar2 = "bar"
      }
    },
    {
      name = "Group B"
    },
    {
      name = "Group C"
    },
  ]
  hosts = [
    {
      name = "Host A"
      description = "A new host for testing"
      groups = [ "Group A", "Group B" ]
      variables = {
        hostvar1 = "foo"
        hostvar2 = "bar"
      }
    },
    {
      name = "Host B"
    },
    {
      name = "Host C"
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
