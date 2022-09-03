# terraform-controller

This page help you confirming of how terraform-controller using terraform-provider-hashicups works as this tutorial

## Prerequisites

The following tools are required to run this tutorial:

- [yq (Lightweight and portable command-line YAML, JSON and XML processor)](https://github.com/mikefarah/yq)
- Needs to install HashiCups provider as [terraform-provider-hashicups](https://github.com/hashicorp/terraform-provider-hashicups)

## How to confirm tutorial of terraform-provider-hashicups

### (1) Preparing for the tutorial environment as [Perform CRUD Operations with Providers](https://learn.hashicorp.com/tutorials/terraform/provider-use?in=terraform/providers)

Initialize HashiCups locally

    cd examples/hashicups/docker-compose  
    docker-compose up

Create new HashiCups user

    curl -X POST localhost:19090/signup -d '{"username":"education", "password":"test123"}'

### (2) Starting terraform-controller as following

    $ go run main.go

    I0826 17:17:05.661724   80521 controller.go:97] Starting  provider controller
    I0826 17:17:05.661721   80521 controller.go:97] Starting  configuration controller

### (3) Creating Secret for credential

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

### (4) Creating Provider for terraform-provider-hashicups

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

### (5) Creating Configuration for applying main.tf to Teraform Core

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

### (6) Confirming result of terraform apply

Let's check if terraform worked fine

    $ cd work
    $ terraform state show hashicups_order.edu

    # hashicups_order.edu:
    resource "hashicups_order" "edu" {
        id = "1"

        items {
            quantity = 2

            coffee {
                id     = 3
                image  = "/nomad.png"
                name   = "Nomadicano"
                price  = 150
                teaser = "Drink one today and you will want to schedule another"
            }
        }
        items {
            quantity = 2

            coffee {
                id     = 2
                image  = "/vault.png"
                name   = "Vaulatte"
                price  = 200
                teaser = "Nothing gives you a safe and secure feeling like a Vaulatte"
            }
        }
    }

### (7) Updating Configuration for applying main.tf to Teraform Core

Let's change quantiites (2->3, 2->1) in configuration
And then, you can confirm content of configuration as following

    $ cd ..
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
            quantity = 3
          }
          items {
            coffee {
              id = 2
            }
            quantity = 1
          }
        }

        output "edu_order" {
          value = hashicups_order.edu
        }
      providerRef:
        name: hashicups
        namespace: default

After converting yaml file to json format, you need to handle for updating configuration via PUT method

    $ yq . -o json examples/hashicups/configuration.yaml | curl -X PUT http://localhost:10000/configuration/default/sample-configuration  -d @- | jq .

    {
      "kind": "Configuration",
      "metadata": {
        "name": "sample-configuration",
        "namespace": "default",
        "creationTimestamp": null
      },
      "spec": {
        "hcl": "resource \"hashicups_order\" \"edu\" {\n  items {\n    coffee {\n      id = 3\n    }\n    quantity = 3\n  }\n  items {\n    coffee {\n      id = 2\n    }\n    quantity = 1\n  }\n}\n\noutput \"edu_order\" {\n  value = hashicups_order.edu\n}",
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

### (8) Confirming result of terraform apply

Let's check if terraform worked fine

    $ cd work
    $ terraform state show hashicups_order.edu

    # hashicups_order.edu:
    resource "hashicups_order" "edu" {
        id           = "7"
        last_updated = "Thursday, 01-Sep-22 14:57:56 JST"

        items {
            quantity = 3

            coffee {
                id     = 3
                image  = "/nomad.png"
                name   = "Nomadicano"
                price  = 150
                teaser = "Drink one today and you will want to schedule another"
            }
        }
        items {
            quantity = 1

            coffee {
                id     = 2
                image  = "/vault.png"
                name   = "Vaulatte"
                price  = 200
                teaser = "Nothing gives you a safe and secure feeling like a Vaulatte"
            }
        }
    }

### (9) Deleting Configuration for destroying to Teraform Core

You need to handle for deleting configuration via DELETE method

    $ curl -X DELETE http://localhost:10000/configuration/default/sample-configuration
    $ curl -X GET http://localhost:10000/configurations
    null

### (10) Confirming result of terraform destroy

Let's check if terraform worked fine

    $ terraform state show hashicups_order.edu

    No instance found for the given address!

    This command requires that the address references one specific instance.
    To view the available instances, use "terraform state list". Please modify 
    the address to reference a specific instance.

That's all
