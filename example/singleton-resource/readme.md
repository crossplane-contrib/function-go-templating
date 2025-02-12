# How It Works  

## Overview  
This mechanism leverages Crossplane Composition, Go templating, and management policies to dynamically determine whether a cloud resource should be observed (imported) or created (managed). It uses Crossplane's ExtraResources feature, a powerful Go templating capability, to check if a requested resource already exists in the cloud and handle it accordingly.

1. **ExtraResources** fetch MR information
2. **Imported (Observed Only)** if it already exists in the cloud.  
3. **Created and Managed** if it does not exist.  

The mechanism leverages **Crossplane Composition** and **management policies** to determine whether the resource should be observed or created.  

---

## Step-by-Step Execution  

### 1️⃣ Extracting the Requested Resource Specification  
The template begins by extracting the required resource details from the **observed composite resource**:  

```yaml
{{- $resourceSpec := index .observed.composite.resource.spec.resource }}
```  

- `$resourceSpec` holds information such as `apiVersion`, `kind`, `metadata.name`, and `spec`.  
- A default **ready status (`$readyStatus`)** is set to `false`.  
- An empty **external name (`$externalName`)** is initialized.  

---

### 2️⃣ Checking for Existing Resources  
The logic then checks whether this resource already exists in **extraResources**:  

```yaml
{{- with .extraResources }}
  {{- $someExtraResources := index . "resources" }}
  {{- range $i, $extraResource := $someExtraResources.items }}
    {{- $resourceName := $extraResource.resource.metadata.name }}
```  

For each `extraResource`:  
- If the `metadata.name` matches `$resourceSpec.metadata.name`, it means an instance of this resource may already exist.  
- The **status conditions** are parsed:  

  ```yaml
  {{- $extraResourcesStatus := fromJson ($extraResource.resource.status.conditions | toJson) }}
  ```  

- If there is more than one condition:  
  - The **second condition** (`syncConditionReadyStatus`) is checked to determine if it is `"True"` (i.e., the resource is ready).  
  - If **ready**, `$readyStatus` is set to `true`, and the `external-name` annotation is extracted:  

    ```yaml
    {{ $externalName = index $extraResource.resource.metadata.annotations "crossplane.io/external-name" }}
    ```  

- If the conditions are missing or not `"True"`, `$readyStatus` remains `false`.  

---

### 3️⃣ Handling the Resource Based on Readiness

#### ✅ **Case 1: Resource Already Exists**  
If `$readyStatus` is `true`:  

- A **read-only Crossplane resource** is created, importing the existing cloud resource using an `external-name`:  

  ```yaml
  apiVersion: {{ $resourceSpec.apiVersion }}
  kind: {{ $resourceSpec.kind }}
  metadata:
    name: {{ $resourceSpec.metadata.name }}-readonly-{{ $.observed.composite.resource.metadata.name }}
    annotations:
      crossplane.io/external-name: {{ $externalName }}
  spec:
    managementPolicies: ["Observe"]
  ```  

- A **singleton status resource** (`XSingletonResource`) is updated to indicate that the resource already exists:  

  ```yaml
  apiVersion: example.crossplane.io/v1
  kind: XSingletonResource
  metadata:
    name: {{ $.observed.composite.resource.metadata.name }}
  status:
    externalResourceStatus: "Already exist {{ $resourceSpec.metadata.name }} of kind {{ $resourceSpec.kind }}"
    externalName: {{ $externalName }}
  ```  

---

#### ❌ **Case 2: Resource Does Not Exist**  
If `$readyStatus` is `false`:  

- A **new resource is created and managed** by Crossplane with `managementPolicies: ["Observe", "Create"]`:  

  ```yaml
  apiVersion: {{ $resourceSpec.apiVersion }}
  kind: {{ $resourceSpec.kind }}
  metadata:
    name: {{ $resourceSpec.metadata.name }}
    annotations:
      crossplane.io/external-name: {{ $resourceSpec.metadata.name }}
  spec:
    managementPolicies: ["Observe", "Create"]
  ```  

- A **singleton status resource** (`XSingletonResource`) is updated to indicate that the resource is being created:  

  ```yaml
  apiVersion: example.crossplane.io/v1
  kind: XSingletonResource
  metadata:
    name: {{ $.observed.composite.resource.metadata.name }}
  status:
    externalResourceStatus: "resource name {{ $resourceSpec.metadata.name }} of kind {{ $resourceSpec.kind }} not exist. Creating.."
    externalName: {{ $externalName }}
  ```  

---

## Summary  

| Condition | Action Taken |
|-----------|-------------|
| **Resource exists** in cloud (determined via `extraResources`) | Import as **read-only** with `"Observe"` policy. |
| **Resource does not exist** | Create a new resource and manage it with `"Observe", "Create"` policies. |

### Key Benefits  
✅ Prevents duplicate resource creation in the cloud.  
✅ Ensures the correct resource is always referenced.  
✅ Keeps resources in **read-only mode** to prevent accidental modifications.  
✅ Allows claims to share the same cloud resource while maintaining unique metadata inside Kubernetes.  

---

This explanation should provide a clear understanding of how the template works. Let me know if you need any refinements! 🚀
