// -*- groovy -*-
pipeline {
  agent { label 'ec2-xenial-vendors' }
  options { timeout(time: 20, unit: 'MINUTES') }

  stages {
    stage('Build') {
      steps {
        // Command to install golang-go and s3cmd taken from
        // user-content Jenkins build; I'm assuming it's there because
        // at this point we can't trust Jenkins slave will have them
        // handy.
        // + Added redis.
        // TODO: verify this, move to node configuration if possible.
        sh 'sudo apt-get install --yes golang-go s3cmd redis-server'
        sh 'make all'
      }
    }

    stage('Test') {
      steps {
        sh 'make test'
      }
    }

    stage('Upload') {
      when {
        expression {
          GIT_BRANCH = sh(returnStdout: true, script: 'git rev-parse --abbrev-ref HEAD').trim()
          return GIT_BRANCH == "master"
        }
      }
      steps {
        withCredentials([[$class: 'AmazonWebServicesCredentialsBinding', credentialsId: 'a4b674e4-0521-4734-92fb-831ddcaddb46']]) {
          sh 'make upload'
        }
      }
    }
  }
}