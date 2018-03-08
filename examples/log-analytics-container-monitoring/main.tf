resource "azurerm_resource_group" "test" {
	name     = "k8s-log-analytics-test"
	location = "westeurope"
}

resource "random_id" "workspace" {
  keepers = {
    # Generate a new id each time we switch to a new resource group
    group_name = "${azurerm_resource_group.test.name}"
  }

  byte_length = 8
}

#an attempt to keep the aci container group name (and dns label) somewhat unique
resource "random_integer" "random_int" {
    min = 100
    max = 999
}

resource "azurerm_kubernetes_cluster" "aks" {
    name        = "akc-${random_integer.random_int.result}"
    location    = "${azurerm_resource_group.test.location}"
    dns_prefix  = "akc-${random_integer.random_int.result}"

    resource_group_name = "${azurerm_resource_group.test.name}"
    kubernetes_version  = "1.8.7"


    linux_profile {
        admin_username = "${var.linux_admin_username}"
        ssh_key {
            key_data = "${var.linux_admin_ssh_publickey}"
        }
    }

    agent_pool_profile {
        name        = "agentpool"
        count       = "2"
        vm_size     = "Standard_DS2_v2"
        os_type     = "Linux"
    }

    service_principal {
        client_id     = "${var.client_id}"
        client_secret = "${var.client_secret}"
    }
}
  
resource "azurerm_log_analytics_workspace" "test" {
	name                = "k8s-workspace-${random_id.workspace.hex}"
	location            = "${azurerm_resource_group.test.location}"
	resource_group_name = "${azurerm_resource_group.test.name}"
	sku                 = "Free"
}
  
resource "azurerm_log_analytics_solution" "test" {
	solution_name         = "Containers"
	location              = "${azurerm_resource_group.test.location}"
	resource_group_name   = "${azurerm_resource_group.test.name}"
	workspace_resource_id = "${azurerm_log_analytics_workspace.test.id}"
	workspace_name        = "${azurerm_log_analytics_workspace.test.name}"
	
	plan {
	  publisher      = "Microsoft"
	  product        = "OMSGallery/Containers"
	}
}

provider "kubernetes" {
  host     = "${azurerm_kubernetes_cluster.aks.kube_config.0.host}"
  username = "${azurerm_kubernetes_cluster.aks.kube_config.0.username}"
  password = "${azurerm_kubernetes_cluster.aks.kube_config.0.password}"

  client_certificate     = "${azurerm_kubernetes_cluster.aks.kube_config.0.client_certificate}"
  client_key             = "${azurerm_kubernetes_cluster.aks.kube_config.0.client_key}"
  cluster_ca_certificate = "${azurerm_kubernetes_cluster.aks.kube_config.0.cluster_ca_certificate}"
}

resource "kubernetes_namespace" "monitoring" {
  metadata {
    name = "monitoring"
  }
}
