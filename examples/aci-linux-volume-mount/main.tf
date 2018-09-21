resource "azurerm_resource_group" "aci-rg" {
  name     = "${var.resource_group_name}"
  location = "${var.resource_group_location}"
}

#storage account name needs to be globally unique so lets generate a random id
resource "random_integer" "random_int" {
  min = 100
  max = 999
}

resource "azurerm_storage_account" "aci-sa" {
  name                     = "acistorageacct${random_integer.random_int.result}"
  resource_group_name      = "${azurerm_resource_group.aci-rg.name}"
  location                 = "${azurerm_resource_group.aci-rg.location}"
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_storage_share" "aci-share" {
  name = "aci-test-share"

  resource_group_name  = "${azurerm_resource_group.aci-rg.name}"
  storage_account_name = "${azurerm_storage_account.aci-sa.name}"

  quota = 50
}

resource "azurerm_container_group" "aci-example" {
  name                = "mycontainergroup-${random_integer.random_int.result}"
  location            = "${azurerm_resource_group.aci-rg.location}"
  resource_group_name = "${azurerm_resource_group.aci-rg.name}"
  ip_address_type     = "public"
  dns_name_label      = "mycontainergroup-${random_integer.random_int.result}"
  os_type             = "linux"

  volume {
    name      = "emptydir"
    empty_dir = {}
  }

  volume {
    name      = "secret"
    
    secret = {
      name = "examplesecret0"
      data = "YmFzZTY0IGRhdGEK" // Base64 data saying "base64 data"
    }
    secret = {
      name = "examplesecret1"
      data = "YmFzZTY0IGRhdGEK" // Base64 data saying "base64 data"
    }
    secret = {
      name = "examplesecret2"
      data = "YmFzZTY0IGRhdGEK" // Base64 data saying "base64 data"
    }
  }


  volume {
    name = "azureshare"

    azure_share {
      share_name           = "${azurerm_storage_share.aci-share.name}"
      storage_account_name = "${azurerm_storage_account.aci-sa.name}"
      storage_account_key  = "${azurerm_storage_account.aci-sa.primary_access_key}"
    }
  }

  volume {
    name = "gitrepo"

    git_repo {
      repository = "https://github.com/Azure-Samples/aci-tutorial-sidecar"
    }
  }

  container {
    name     = "webserver"
    image    = "seanmckenna/aci-hellofiles"
    cpu      = "1"
    memory   = "1.5"
    port     = "80"
    protocol = "tcp"

    volume_mount {
      volume_name = "emptydir"
      mount_path  = "/aci/empty"
    }

    volume_mount {
      volume_name = "gitrepo"
      mount_path  = "/aci/gitrepo"
    }

    volume_mount {
      volume_name = "secret"
      mount_path  = "/aci/secret"
    }
  }

  container {
    name   = "sidecar"
    image  = "seanmckenna/aci-hellofiles"
    cpu    = "1"
    memory = "1.5"

    volume_mount {
      volume_name = "emptydir"
      mount_path  = "/empty"
      read_only   = false
    }

    volume_mount {
      volume_name = "gitrepo"
      mount_path  = "/gitrepo"
      read_only   = false
    }

    volume_mount {
      volume_name = "azureshare"
      mount_path  = "/azureshare"
      read_only   = false
    }
  }

  tags {
    environment = "testing"
  }
}
