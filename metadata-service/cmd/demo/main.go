package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"metadata-service/internal/model"
	"metadata-service/internal/repository"
	"metadata-service/internal/store"
)

const demoDBPath = "data/demo.db"

func main() {
	step := flag.Int("step", 0, "which demo step to run (1-14)")
	flag.Parse()

	if *step == 0 {
		fmt.Println("usage: go run ./cmd/demo --step=N   (see README table for N=1..14)")
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s, err := store.Open(demoDBPath)
	if err != nil {
		logger.Error("open store", slog.Any("error", err))
		os.Exit(1)
	}
	defer s.Close()

	buckets := repository.NewBoltBucketRepository(s.DB(), logger)
	objects := repository.NewBoltObjectRepository(s.DB(), logger)
	ctx := context.Background()

	switch *step {

	case 1:
		fmt.Println("--- Step 1: alice creates bucket 'photos' ---")
		err := buckets.Create(ctx, model.Bucket{ID: "b-photos", Name: "photos", OwnerID: "user-alice"})
		printResult("Create(photos, alice)", err)

	case 2:
		fmt.Println("--- Step 2: bob creates bucket 'documents' ---")
		err := buckets.Create(ctx, model.Bucket{ID: "b-documents", Name: "documents", OwnerID: "user-bob"})
		printResult("Create(documents, bob)", err)

	case 3:
		fmt.Println("--- Step 3: bob tries to create 'photos' too (should be rejected) ---")
		err := buckets.Create(ctx, model.Bucket{ID: "b-photos-2", Name: "photos", OwnerID: "user-bob"})
		printResult("Create(photos, bob) — expect ErrAlreadyExists", err)
		fmt.Println("Is ErrAlreadyExists:", errors.Is(err, repository.ErrAlreadyExists))

	case 4:
		fmt.Println("--- Step 4: alice puts two objects into 'photos' ---")
		err1 := objects.Put(ctx, model.Object{
			ID: "o-vacation", BucketName: "photos", ObjectKey: "vacation.png",
			SizeBytes: 204800, ETag: "etag-vacation", ContentType: "image/png",
			StorageRef: "photos/vacation.png", VersionID: "null", IsLatest: true,
		})
		printResult("Put(photos/vacation.png)", err1)

		err2 := objects.Put(ctx, model.Object{
			ID: "o-profile", BucketName: "photos", ObjectKey: "profile.jpg",
			SizeBytes: 51200, ETag: "etag-profile", ContentType: "image/jpeg",
			StorageRef: "photos/profile.jpg", VersionID: "null", IsLatest: true,
		})
		printResult("Put(photos/profile.jpg)", err2)

	case 5:
		fmt.Println("--- Step 5: bob puts one object into 'documents' ---")
		err := objects.Put(ctx, model.Object{
			ID: "o-readme", BucketName: "documents", ObjectKey: "readme.txt",
			SizeBytes: 1024, ETag: "etag-readme", ContentType: "text/plain",
			StorageRef: "documents/readme.txt", VersionID: "null", IsLatest: true,
		})
		printResult("Put(documents/readme.txt)", err)

	case 6:
		fmt.Println("--- Step 6: Get a single object ---")
		obj, err := objects.Get(ctx, "photos", "vacation.png")
		printResult("Get(photos/vacation.png)", err)
		if err == nil {
			fmt.Printf("  -> %+v\n", obj)
		}

	case 7:
		fmt.Println("--- Step 7: List all objects in 'photos' ---")
		list, err := objects.List(ctx, "photos", "", 100)
		printResult("List(photos, prefix=\"\")", err)
		for _, o := range list {
			fmt.Printf("  -> %s (%d bytes)\n", o.ObjectKey, o.SizeBytes)
		}

	case 8:
		fmt.Println("--- Step 8: ListByOwner for both users ---")
		aliceBuckets, err := buckets.ListByOwner(ctx, "user-alice")
		printResult("ListByOwner(user-alice)", err)
		for _, b := range aliceBuckets {
			fmt.Printf("  alice owns: %s\n", b.Name)
		}

		bobBuckets, err := buckets.ListByOwner(ctx, "user-bob")
		printResult("ListByOwner(user-bob)", err)
		for _, b := range bobBuckets {
			fmt.Printf("  bob owns: %s\n", b.Name)
		}

	case 9:
		fmt.Println("--- Step 9: Delete one object (soft delete) ---")
		err := objects.Delete(ctx, "photos", "profile.jpg")
		printResult("Delete(photos/profile.jpg)", err)

		list, _ := objects.List(ctx, "photos", "", 100)
		fmt.Println("  photos now lists:")
		for _, o := range list {
			fmt.Printf("    -> %s\n", o.ObjectKey)
		}

	case 10:
		fmt.Println("--- Step 10: alice tries to delete 'photos' — should fail, vacation.png still live ---")
		err := buckets.Delete(ctx, "photos")
		printResult("Delete(photos) — expect ErrBucketNotEmpty", err)
		fmt.Println("Is ErrBucketNotEmpty:", errors.Is(err, repository.ErrBucketNotEmpty))

	case 11:
		fmt.Println("--- Step 11: delete the last live object, then delete the bucket ---")
		err1 := objects.Delete(ctx, "photos", "vacation.png")
		printResult("Delete(photos/vacation.png)", err1)

		err2 := buckets.Delete(ctx, "photos")
		printResult("Delete(photos) — should now succeed", err2)

	case 12:
		fmt.Println("--- Step 12: confirm alice's bucket list is now empty ---")
		aliceBuckets, err := buckets.ListByOwner(ctx, "user-alice")
		printResult("ListByOwner(user-alice)", err)
		fmt.Printf("  alice owns %d buckets\n", len(aliceBuckets))

	case 13:
		fmt.Println("--- Step 13: bob resurrects the soft-deleted 'photos' name ---")
		err := buckets.Create(ctx, model.Bucket{ID: "b-photos-resurrected", Name: "photos", OwnerID: "user-bob"})
		printResult("Create(photos, bob) over soft-deleted name", err)

	case 14:
		fmt.Println("--- Step 14: regression check — alice must NOT see bob's resurrected 'photos' ---")
		aliceBuckets, err := buckets.ListByOwner(ctx, "user-alice")
		printResult("ListByOwner(user-alice)", err)
		fmt.Printf("  alice owns %d buckets (want 0)\n", len(aliceBuckets))

		bobBuckets, err := buckets.ListByOwner(ctx, "user-bob")
		printResult("ListByOwner(user-bob)", err)
		for _, b := range bobBuckets {
			fmt.Printf("  bob owns: %s\n", b.Name)
		}

	default:
		fmt.Printf("no such step: %d\n", *step)
		os.Exit(1)
	}
}

func printResult(label string, err error) {
	if err != nil {
		fmt.Printf("%s -> ERROR: %v\n", label, err)
		return
	}
	fmt.Printf("%s -> OK\n", label)
}
