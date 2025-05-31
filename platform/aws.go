/*
 * Copyright 2024-2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package platform

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"io"
	"os"
	"strings"

	"filippo.io/age"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3GetEncryptedBlob retrieves and decrypts the blob to the given file.
func S3GetEncryptedBlob(ctx context.Context, folderName, blobName string, outStream io.Writer) error {
	env := GetConfig()
	myself, err := age.ParseX25519Identity(env.AgeSecretKey)
	if err != nil {
		return err
	}
	client, err := s3GetClient(&env)
	if err != nil {
		return err
	}
	path := blobName
	if folderName != "" {
		path = folderName + "/" + blobName
	}
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.AwsBucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	blobStream, err := age.Decrypt(resp.Body, myself)
	if err != nil {
		return err
	}
	_, err = io.Copy(outStream, blobStream)
	return err
}

// S3PutEncryptedBlob puts the contents of the given file, encrypted, to the given blobName.
func S3PutEncryptedBlob(ctx context.Context, folderName, blobName string, inStream io.Reader) error {
	env := GetConfig()
	myself, err := age.ParseX25519Recipient(env.AgePublicKey)
	if err != nil {
		return err
	}
	client, err := s3GetClient(&env)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", blobName+"-*.age")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	encryptedWriter, err := age.Encrypt(f, myself)
	if err != nil {
		return err
	}
	_, err = io.Copy(encryptedWriter, inStream)
	_ = encryptedWriter.Close()
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	blobLen := stat.Size()
	path := blobName
	if folderName != "" {
		path = folderName + "/" + blobName
	}
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(env.AwsBucket),
		Key:           aws.String(path),
		ContentType:   aws.String("application/octet-stream"),
		ContentLength: aws.Int64(blobLen),
		Body:          f,
	})
	return err
}

func S3DeleteBlob(ctx context.Context, folderName, blobName string) error {
	env := GetConfig()
	client, err := s3GetClient(&env)
	if err != nil {
		return err
	}
	path := blobName
	if folderName != "" {
		path = folderName + "/" + blobName
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.AwsBucket),
		Key:    aws.String(path),
	})
	return err
}

func S3ListBlobs(ctx context.Context, folderName string) ([]string, error) {
	env := GetConfig()
	client, err := s3GetClient(&env)
	if err != nil {
		return nil, err
	}
	var prefix string
	spec := &s3.ListObjectsV2Input{Bucket: aws.String(env.AwsBucket)}
	if folderName != "" {
		prefix = folderName + "/"
		spec = &s3.ListObjectsV2Input{
			Bucket: aws.String(env.AwsBucket),
			Prefix: aws.String(prefix),
		}
	}
	resp, err := client.ListObjectsV2(ctx, spec)
	if err != nil {
		return nil, err
	}
	var blobNames []string
	for _, obj := range resp.Contents {
		name := strings.TrimPrefix(*obj.Key, prefix)
		if name != "" {
			blobNames = append(blobNames, name)
		}
	}
	return blobNames, nil
}

func s3GetClient(env *Environment) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{AccessKeyID: env.AwsAccessKey, SecretAccessKey: env.AwsSecretKey},
		}),
		config.WithRegion(GetConfig().AwsRegion))
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg), nil
}
