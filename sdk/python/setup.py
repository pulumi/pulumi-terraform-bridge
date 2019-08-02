from setuptools import setup, find_packages
from setuptools.command.install import install
from subprocess import check_call

import errno


class InstallPluginCommand(install):
    def run(self):
        install.run(self)
        try:
            check_call(['pulumi', 'plugin', 'install', 'resource', 'terraform', '${PLUGIN_VERSION}'])
        except OSError as error:
            if error.errno == errno.ENOENT:
                print("""
                There was an error installing the terraform resource provider plugin. It looks
                like `pulumi` is not installed on your system.
                Please visit https://pulumi.com to install the Pulumi CLI.
                You may try manually installin the plugin by running:
                `pulumi plugin install resource terraform ${PLUGIN_VERSION}`
                """)


def readme():
    with open('README.rst') as f:
        return f.read()


setup(name='pulumi_terraform',
      version='${VERSION}',
      description='A Pulumi package for consuming Terraform Remote State resources.',
      long_description=readme(),
      cmdclass={
          'install': InstallPluginCommand,
      },
      keywords='pulumi terraform',
      url='https://pulumi.io',
      project_urls={
          'Repository': 'https://github.com/pulumi/pulumi-terraform'
      },
      license='Apache-2.0',
      packages=find_packages(),
      install_requires=[
          'parver>=0.2.1',
          'pulumi>=0.17.24,<0.18.0',
          'semver>=2.8.1'
      ],
      zip_safe=False)
