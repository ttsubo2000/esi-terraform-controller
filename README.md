# esi-terraform-worker

This page help you confirming of how esi-terraform-worker using terraform-provider-hashicups works as this tutorial

## Prerequisites

The following tools are required to run this tutorial:

- [yq (lightweight and portable command-line YAML, JSON and XML processor)](https://github.com/mikefarah/yq)
- [terraform-provider-hashicups](https://github.com/hashicorp/terraform-provider-hashicups)

## How to Run

Starting esi-terraform-worker as following

    $ go run main.go

    I0826 17:17:05.661724   80521 controller.go:97] Starting  provider controller
    I0826 17:17:05.661721   80521 controller.go:97] Starting  configuration controller
    I0826 17:17:06.662009   80521 store.go:104] Update key:[Secret/hashicups/hashicups-account-creds], obj:[&{{Secret } {hashicups-account-creds  hashicups    0 0001-01-01 00:00:00 +0000 UTC <nil> <nil> map[] map[] [] []  []} <nil> map[credentials:HashicupsUser: education
    HashicupsPassword: test123] map[] }]
    I0826 17:17:07.662614   80521 store.go:98] Update key:[Provider/default/hashicups], obj:[&{{Provider } {hashicups  default    0 0001-01-01 00:00:00 +0000 UTC <nil> <nil> map[] map[] [] []  []} {hashicups  {Secret {{hashicups-account-creds hashicups} credentials}}} { }}]
    I0826 17:17:07.662913   80521 provider_controller.go:26] "reconciling Terraform Provider..." NamespacedName="default/hashicups"

### (1) How to create secret

You can confirm content of secret as following

    $ cat examples/hashicups/secret.yaml

    kind: Secret
    metadata:
      name: hashicups-account-creds
      namespace: hashicups
    data:
      credentials: |-
        HashicupsUser: education
        HashicupsPassword: test123

After converting yaml file to json format, you need to handle for creating new secret via POST method

    $ yq . -o json examples/hashicups/secret.yaml | curl -X POST http://localhost:10000/secret -d @- | jq .

    {
      "kind": "Secret",
      "metadata": {
        "name": "hashicups-account-creds",
        "namespace": "hashicups",
        "creationTimestamp": null
      },
      "data": {
        "credentials": "HashicupsUser: education\nHashicupsPassword: test123"
      }
    }

### (2) How to create provider

You can confirm content of secret as following

    $ cat examples/hashicups/provider.yaml 

    kind: Provider
    metadata:
      name: hashicups
      namespace: default
    spec:
      provider: hashicups
      credentials:
        source: Secret
        secretRef:
          name: hashicups-account-creds
          namespace: hashicups
          key: credentials

After converting yaml file to json format, you need to handle for creating new provider via POST method

    $ yq . -o json examples/hashicups/provider.yaml | curl -X POST http://localhost:10000/provider -d @- | jq .

    {
      "kind": "Provider",
      "metadata": {
        "name": "hashicups",
        "namespace": "default",
        "creationTimestamp": null
      },
      "spec": {
        "provider": "hashicups",
        "credentials": {
          "source": "Secret",
          "secretRef": {
            "name": "hashicups-account-creds",
            "namespace": "hashicups",
            "key": "credentials"
          }
        }
      },
      "status": {}
    }

### (3) How to create configuration

You can confirm content of configuration as following

    $ cat examples/hashicups/configuration.yaml 

    kind: Configuration
    metadata:
      name: sample-configuration
      namespace: default
    spec:
      hcl: |-
        resource "hashicups_order" "edu" {
          items {
            coffee {
              id = 3
            }
            quantity = 2
          }
          items {
            coffee {
              id = 2
            }
            quantity = 2
          }
        }

        output "edu_order" {
          value = hashicups_order.edu
        }
      providerRef:
        name: hashicups
        namespace: default

After converting yaml file to json format, you need to handle for creating new configuration via POST method

    $ yq . -o json examples/hashicups/configuration.yaml | curl -X POST http://localhost:10000/configuration -d @- | jq .

    {
      "kind": "Configuration",
      "metadata": {
        "name": "sample-configuration",
        "namespace": "default",
        "creationTimestamp": null
      },
      "spec": {
        "hcl": "resource \"hashicups_order\" \"edu\" {\n  items {\n    coffee {\n      id = 3\n    }\n    quantity = 2\n  }\n  items {\n    coffee {\n      id = 2\n    }\n    quantity = 2\n  }\n}\n\noutput \"edu_order\" {\n  value = hashicups_order.edu\n}",
        "providerRef": {
          "name": "hashicups",
          "namespace": "default"
        }
      },
      "status": {
        "apply": {},
        "destroy": {}
      }
    }
