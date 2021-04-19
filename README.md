# Android SDK Add-on Installer

This step will download a specific version of the Android SDK Add-on into the Bitrise VM, allowing apps to build its code linking against this add-on code


## How to use this Step

This step can be configured using:

- **add_on_url:** URL to the XML file with the Add on definition
- **android_sdk_path:** Path to the Android SDK in the Virtual Machine (VM)
- **verbose_log:** Flag to enable more information while running the step

Output:

- **ADD_ON_SDK_PATH:** This is the path where the Add on was installed, relative to `android_sdk_path`

```yaml
- git::https://github.com/FutureWorkshops/bitrise-step-android-sdk-add-on-installer.git@main:
   title: Install Add On
   inputs:
   - add_on_url: $ADD_ON_URL
   - android_sdk_path: $ANDROID_HOME
   - verbose_log: "no"
```
