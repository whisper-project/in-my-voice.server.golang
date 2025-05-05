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

	"filippo.io/age"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3GetEncryptedBlob retrieves and decrypts the blob to the given file.
func S3GetEncryptedBlob(ctx context.Context, blobName string, outStream io.Writer) error {
	env := GetConfig()
	myself, err := age.ParseX25519Identity(env.AgeSecretKey)
	if err != nil {
		return err
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{AccessKeyID: env.AwsAccessKey, SecretAccessKey: env.AwsSecretKey},
		}),
		config.WithRegion(GetConfig().AwsRegion))
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.AwsBucket),
		Key:    aws.String(env.AwsFolder + "/" + blobName),
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
func S3PutEncryptedBlob(ctx context.Context, blobName string, inStream io.Reader) error {
	env := GetConfig()
	myself, err := age.ParseX25519Recipient(env.AgePublicKey)
	if err != nil {
		return err
	}
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{AccessKeyID: env.AwsAccessKey, SecretAccessKey: env.AwsSecretKey},
		}),
		config.WithRegion(GetConfig().AwsRegion))
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
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
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(env.AwsBucket),
		Key:           aws.String(env.AwsFolder + "/" + blobName),
		ContentType:   aws.String("application/octet-stream"),
		ContentLength: aws.Int64(blobLen),
		Body:          f,
	})
	return err
}

func S3DeleteBlob(ctx context.Context, blobName string) error {
	env := GetConfig()
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{AccessKeyID: env.AwsAccessKey, SecretAccessKey: env.AwsSecretKey},
		}),
		config.WithRegion(GetConfig().AwsRegion))
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.AwsBucket),
		Key:    aws.String(env.AwsFolder + "/" + blobName),
	})
	return err
}
