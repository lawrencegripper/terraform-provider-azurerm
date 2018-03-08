package azurerm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2017-09-30/containerservice"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
	yaml "gopkg.in/yaml.v2"
)

func resourceArmKubernetesCluster() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmKubernetesClusterCreate,
		Read:   resourceArmKubernetesClusterRead,
		Update: resourceArmKubernetesClusterCreate,
		Delete: resourceArmKubernetesClusterDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"location": locationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"dns_prefix": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"fqdn": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"kubernetes_version": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"kube_config": {
				Type:     schema.TypeList,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"kubeconfig": {
							Type:     schema.TypeString,
							Required: true,
						},
						"username": {
							Type:     schema.TypeString,
							Required: true,
						},
						"password": {
							Type:     schema.TypeString,
							Required: true,
						},
						"clientCertificate": {
							Type:     schema.TypeString,
							Required: true,
						},
						"CA": {
							Type:     schema.TypeString,
							Required: true,
						"host": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"username": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"password": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"client_certificate": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"client_key": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"cluster_ca_certificate": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},

			"linux_profile": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"admin_username": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"ssh_key": {
							Type:     schema.TypeList,
							Required: true,
							ForceNew: true,

							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"key_data": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
								},
							},
						},
					},
				},
			},

			"agent_pool_profile": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},

						"count": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      1,
							ValidateFunc: validation.IntBetween(1, 50),
						},

						"dns_prefix": {
							Type:     schema.TypeString,
							Computed: true,
						},

						"fqdn": {
							Type:       schema.TypeString,
							Computed:   true,
							Deprecated: "This field has been deprecated. Use the parent `fqdn` instead",
						},

						"vm_size": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},

						"os_disk_size_gb": {
							Type:     schema.TypeInt,
							Optional: true,
							ForceNew: true,
						},

						"vnet_subnet_id": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},

						"os_type": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
							Default:  containerservice.Linux,
							ValidateFunc: validation.StringInSlice([]string{
								string(containerservice.Linux),
								string(containerservice.Windows),
							}, true),
							DiffSuppressFunc: ignoreCaseDiffSuppressFunc,
						},
					},
				},
			},

			"service_principal": {
				Type:     schema.TypeSet,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"client_id": {
							Type:     schema.TypeString,
							Required: true,
						},

						"client_secret": {
							Type:      schema.TypeString,
							Required:  true,
							Sensitive: true,
						},
					},
				},
				Set: resourceAzureRMKubernetesClusterServicePrincipalProfileHash,
			},

			"tags": tagsSchema(),
		},
	}
}

func resourceArmKubernetesClusterCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient)
	kubernetesClustersClient := client.kubernetesClustersClient

	log.Printf("[INFO] preparing arguments for Azure ARM AKS managed cluster creation.")

	resGroup := d.Get("resource_group_name").(string)
	name := d.Get("name").(string)
	location := d.Get("location").(string)
	dnsPrefix := d.Get("dns_prefix").(string)
	kubernetesVersion := d.Get("kubernetes_version").(string)

	linuxProfile := expandAzureRmKubernetesClusterLinuxProfile(d)
	agentProfiles := expandAzureRmKubernetesClusterAgentProfiles(d)
	servicePrincipalProfile := expandAzureRmKubernetesClusterServicePrincipal(d)

	tags := d.Get("tags").(map[string]interface{})

	parameters := containerservice.ManagedCluster{
		Name:     &name,
		Location: &location,
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			AgentPoolProfiles:       &agentProfiles,
			DNSPrefix:               &dnsPrefix,
			KubernetesVersion:       &kubernetesVersion,
			LinuxProfile:            &linuxProfile,
			ServicePrincipalProfile: servicePrincipalProfile,
		},
		Tags: expandTags(tags),
	}

	ctx := client.StopContext
	future, err := kubernetesClustersClient.CreateOrUpdate(ctx, resGroup, name, parameters)
	if err != nil {
		return err
	}

	err = future.WaitForCompletion(ctx, kubernetesClustersClient.Client)
	if err != nil {
		return err
	}

	read, err := kubernetesClustersClient.Get(ctx, resGroup, name)
	if err != nil {
		return err
	}

	if read.ID == nil {
		return fmt.Errorf("Cannot read AKS Managed Cluster %q (Resource Group %q) ID", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmKubernetesClusterRead(d, meta)
}

func resourceArmKubernetesClusterRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient)
	kubernetesClustersClient := meta.(*ArmClient).kubernetesClustersClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	name := id.Path["managedClusters"]

	ctx := client.StopContext
	resp, err := kubernetesClustersClient.Get(ctx, resGroup, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("Error making Read request on AKS Managed Cluster %q (resource group %q): %+v", name, resGroup, err)
	}

	d.Set("name", resp.Name)
	if location := resp.Location; location != nil {
		d.Set("location", *location)
	}
	d.Set("resource_group_name", resGroup)
	d.Set("dns_prefix", resp.DNSPrefix)
	d.Set("fqdn", resp.Fqdn)
	d.Set("kubernetes_version", resp.KubernetesVersion)

	linuxProfile := flattenAzureRmKubernetesClusterLinuxProfile(*resp.ManagedClusterProperties.LinuxProfile)
	if err := d.Set("linux_profile", &linuxProfile); err != nil {
		return fmt.Errorf("Error setting `linux_profile`: %+v", err)
	}

	agentPoolProfiles := flattenAzureRmKubernetesClusterAgentPoolProfiles(resp.ManagedClusterProperties.AgentPoolProfiles, resp.Fqdn)
	if err := d.Set("agent_pool_profile", &agentPoolProfiles); err != nil {
		return fmt.Errorf("Error setting `agent_pool_profile`: %+v", err)
	}

	servicePrincipal := flattenAzureRmKubernetesClusterServicePrincipalProfile(resp.ManagedClusterProperties.ServicePrincipalProfile)
	if servicePrincipal != nil {
		d.Set("service_principal", servicePrincipal)
	}

	profile, err := kubernetesClustersClient.GetAccessProfiles(ctx, resGroup, name, "clusterUser")
	if err != nil {
		return fmt.Errorf("Error getting access profile while making Read request on AKS Managed Cluster %q (resource group %q): %+v", name, resGroup, err)
	}

	kubeConfig := flattenAzureRmKubernetesClusterAccessProfile(&profile)
	if kubeConfig != nil {
		if err := d.Set("kube_config", &kubeConfig); err != nil {
			return fmt.Errorf("Error setting `kube_config`: %+v", err)
		}
	}

	flattenAndSetTags(d, resp.Tags)

	return nil
}

func resourceArmKubernetesClusterDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient)
	kubernetesClustersClient := client.kubernetesClustersClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}
	resGroup := id.ResourceGroup
	name := id.Path["managedClusters"]

	ctx := client.StopContext
	future, err := kubernetesClustersClient.Delete(ctx, resGroup, name)
	if err != nil {
		return fmt.Errorf("Error issuing AzureRM delete request of AKS Managed Cluster %q (resource Group %q): %+v", name, resGroup, err)
	}

	return future.WaitForCompletion(ctx, kubernetesClustersClient.Client)
}

func flattenAzureRmKubernetesClusterLinuxProfile(profile containerservice.LinuxProfile) []interface{} {
	profiles := make([]interface{}, 0)
	values := make(map[string]interface{})
	sshKeys := make([]interface{}, 0, len(*profile.SSH.PublicKeys))

	for _, ssh := range *profile.SSH.PublicKeys {
		keys := make(map[string]interface{})
		keys["key_data"] = *ssh.KeyData
		sshKeys = append(sshKeys, keys)
	}

	values["admin_username"] = *profile.AdminUsername
	values["ssh_key"] = sshKeys

	profiles = append(profiles, values)

	return profiles
}

func flattenAzureRmKubernetesClusterAgentPoolProfiles(profiles *[]containerservice.AgentPoolProfile, fqdn *string) []interface{} {
	agentPoolProfiles := make([]interface{}, 0, len(*profiles))

	for _, profile := range *profiles {
		agentPoolProfile := make(map[string]interface{})

		if profile.Count != nil {
			agentPoolProfile["count"] = int(*profile.Count)
		}

		if profile.DNSPrefix != nil {
			agentPoolProfile["dns_prefix"] = *profile.DNSPrefix
		}

		if fqdn != nil {
			// temporarily persist the parent FQDN here until `fqdn` is removed from the `agent_pool_profile`
			agentPoolProfile["fqdn"] = *fqdn
		}

		if profile.Name != nil {
			agentPoolProfile["name"] = *profile.Name
		}

		if profile.VMSize != "" {
			agentPoolProfile["vm_size"] = string(profile.VMSize)
		}

		if profile.OsDiskSizeGB != nil {
			agentPoolProfile["os_disk_size_gb"] = int(*profile.OsDiskSizeGB)
		}

		if profile.VnetSubnetID != nil {
			agentPoolProfile["vnet_subnet_id"] = *profile.VnetSubnetID
		}

		if profile.OsType != "" {
			agentPoolProfile["os_type"] = string(profile.OsType)
		}

		agentPoolProfiles = append(agentPoolProfiles, agentPoolProfile)
	}

	return agentPoolProfiles
}

func flattenAzureRmKubernetesClusterServicePrincipalProfile(profile *containerservice.ServicePrincipalProfile) *schema.Set {
	if profile == nil {
		return nil
	}

	servicePrincipalProfiles := &schema.Set{
		F: resourceAzureRMKubernetesClusterServicePrincipalProfileHash,
	}

	values := make(map[string]interface{})

	values["client_id"] = *profile.ClientID
	if profile.Secret != nil {
		values["client_secret"] = *profile.Secret
	}

	servicePrincipalProfiles.Add(values)

	return servicePrincipalProfiles
}

func flattenAzureRmKubernetesClusterAccessProfile(profile *containerservice.ManagedClusterAccessProfile) []interface{} {
	if profile != nil {
		accessProfile := profile.AccessProfile
		if accessProfile != nil {
			if accessProfile.KubeConfig != nil {
				kubeConfig := getKubeConfig(accessProfile.KubeConfig)
				if kubeConfig != nil {
					kubeConfigFlat := flattenKubeConfig(kubeConfig)
					return kubeConfigFlat
				}
			}
		}
	}
	return nil
}

func flattenKubeConfig(config *KubeConfig) []interface{} {
	if config == nil {
		return nil
	}

	profiles := make([]interface{}, 0)
	values := make(map[string]interface{})
	cluster := config.Clusters[0].Cluster
	user := config.Users[0].User
	name := config.Users[0].Name
	values["host"] = cluster.Server
	values["username"] = name
	values["password"] = user.Token
	values["client_certificate"] = user.ClientCertificteData
	values["client_key"] = user.ClientKeyData
	values["cluster_ca_certificate"] = cluster.ClusterAuthorityData

	profiles = append(profiles, values)
	return profiles
}

func expandAzureRmKubernetesClusterLinuxProfile(d *schema.ResourceData) containerservice.LinuxProfile {
	profiles := d.Get("linux_profile").([]interface{})
	config := profiles[0].(map[string]interface{})

	adminUsername := config["admin_username"].(string)
	linuxKeys := config["ssh_key"].([]interface{})

	keyData := ""
	if key, ok := linuxKeys[0].(map[string]interface{}); ok {
		keyData = key["key_data"].(string)
	}
	sshPublicKey := containerservice.SSHPublicKey{
		KeyData: &keyData,
	}

	sshPublicKeys := []containerservice.SSHPublicKey{sshPublicKey}

	profile := containerservice.LinuxProfile{
		AdminUsername: &adminUsername,
		SSH: &containerservice.SSHConfiguration{
			PublicKeys: &sshPublicKeys,
		},
	}

	return profile
}

func expandAzureRmKubernetesClusterServicePrincipal(d *schema.ResourceData) *containerservice.ServicePrincipalProfile {
	value, exists := d.GetOk("service_principal")
	if !exists {
		return nil
	}

	configs := value.(*schema.Set).List()

	config := configs[0].(map[string]interface{})

	clientId := config["client_id"].(string)
	clientSecret := config["client_secret"].(string)

	principal := containerservice.ServicePrincipalProfile{
		ClientID: &clientId,
		Secret:   &clientSecret,
	}

	return &principal
}

func expandAzureRmKubernetesClusterAgentProfiles(d *schema.ResourceData) []containerservice.AgentPoolProfile {
	configs := d.Get("agent_pool_profile").([]interface{})
	config := configs[0].(map[string]interface{})
	profiles := make([]containerservice.AgentPoolProfile, 0, len(configs))

	name := config["name"].(string)
	count := int32(config["count"].(int))
	dnsPrefix := config["dns_prefix"].(string)
	vmSize := config["vm_size"].(string)
	osDiskSizeGB := int32(config["os_disk_size_gb"].(int))
	vnetSubnetID := config["vnet_subnet_id"].(string)
	osType := config["os_type"].(string)

	profile := containerservice.AgentPoolProfile{
		Name:           &name,
		Count:          &count,
		VMSize:         containerservice.VMSizeTypes(vmSize),
		DNSPrefix:      &dnsPrefix,
		OsDiskSizeGB:   &osDiskSizeGB,
		StorageProfile: containerservice.ManagedDisks,
		VnetSubnetID:   &vnetSubnetID,
		OsType:         containerservice.OSType(osType),
	}

	profiles = append(profiles, profile)

	return profiles
}

func resourceAzureRMKubernetesClusterServicePrincipalProfileHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	clientId := m["client_id"].(string)
	buf.WriteString(fmt.Sprintf("%s-", clientId))

	return hashcode.String(buf.String())
}

func base64Decode(str string) (string, bool) {
	data, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", true
	}
	return string(data), false
}

func getKubeConfig(config *string) *KubeConfig {
	if config == nil {
		return nil
	}

	configStr, error := base64Decode(*config)
	if error == false && configStr != "" {
		log.Println(config)
		var kubeConfig KubeConfig
		err := yaml.Unmarshal([]byte(configStr), &kubeConfig)
		if err == nil && len(kubeConfig.Clusters) > 0 && len(kubeConfig.Users) > 0 {
			return &kubeConfig
		}
	}

	return nil
}

//TODO: Hide these
type ClusterItem struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

type Cluster struct {
	ClusterAuthorityData string `yaml:"certificate-authority-data"`
	Server               string `yaml:"server"`
}

type UserItem struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

type User struct {
	ClientCertificteData string `yaml:"client-certificate-data"`
	Token                string `yaml:"token"`
	ClientKeyData        string `yaml:"client-key-data"`
}

type ContextItem struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

type Context struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type KubeConfig struct {
	APIVersion     string                 `yaml:"apiVersion"`
	Clusters       []ClusterItem          `yaml:"clusters"`
	Users          []UserItem             `yaml:"users"`
	Contexts       []ContextItem          `yaml:"contexts,omitempty"`
	CurrentContext string                 `yaml:"current-context,omitempty"`
	Kind           string                 `yaml:"kind,omitempty"`
	Preferences    map[string]interface{} `yaml:"preferences,omitempty"`
}
