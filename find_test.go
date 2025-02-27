package test_main

import (
	"testing"

	. "github.com/bjartek/overflow"
	"github.com/hexops/autogold"
	"github.com/stretchr/testify/assert"
)

/*
Tests must be in the same folder as flow.json with contracts and transactions/scripts in subdirectories in order for the path resolver to work correctly
*/
func TestFIND(t *testing.T) {
	otu := NewOverflowTest(t).
		setupFIND().
		createUser(100.0, "user1").
		registerUser("user1")

	t.Run("Should be able to register a name", func(t *testing.T) {
		// Can fix this with pointerWant
		otu.O.Script("getLeases").AssertWithPointerWant(t, "/0/name",
			autogold.Want("allLeases", "user1"),
		)
	})

	t.Run("Should get error if you try to register a name and dont have enough money", func(t *testing.T) {
		otu.O.Tx("register",
			WithSigner("user1"),
			WithArg("name", "usr"),
			WithArg("maxAmount", 1001.0),
		).Print().AssertFailure(t, "Balance of vault is not high enough")
	})

	t.Run("Should get error if you try to register a name that is too short", func(t *testing.T) {
		otu.O.Tx("register",
			WithSigner("user1"),
			WithArg("name", "ur"),
			WithArg("maxAmount", 5.0/0.42),
		).AssertFailure(t, "A FIND name has to be lower-cased alphanumeric or dashes and between 3 and 16 characters")
	})

	t.Run("Should get error if you try to register a name that is already claimed", func(t *testing.T) {
		otu.O.Tx("register",
			WithSigner("user1"),
			WithArg("name", "user1"),
			WithArg("maxAmount", 5.0/0.42),
		).AssertFailure(t, "Name already registered")
	})

	t.Run("Should allow registering a lease after it is freed", func(t *testing.T) {
		otu.expireLease().tickClock(2.0)

		otu.O.Tx(`
			import FIND from "../contracts/FIND.cdc"

			transaction(name: String) {

			    prepare(account: AuthAccount) {
			        let status=FIND.status(name)
							if status.status == FIND.LeaseStatus.LOCKED {
								panic("locked")
							}
							if status.status == FIND.LeaseStatus.FREE {
								panic("free")
							}
			    }
			}
			`,
			WithSigner("user1"),
			WithArg("name", "user1"),
		).AssertFailure(t, "locked").
			AssertComputationLessThenOrEqual(t, 1000)

		otu.expireLease()
		otu.registerUser("user1")
	})

	t.Run("Should be able to lookup address", func(t *testing.T) {
		otu.assertLookupAddress("user1", otu.O.Address("user1"))
	})

	t.Run("Should not be able to lookup lease after expired", func(t *testing.T) {
		otu.expireLease().
			tickClock(2.0)

		otu.O.Script("getNameStatus",
			WithArg("name", "user1"),
		).
			AssertWant(t, autogold.Want("getNameStatus n", nil))
	})

	t.Run("Admin should be able to register without paying FUSD", func(t *testing.T) {
		otu.O.Tx("adminRegisterName",
			WithSigner("find-admin"),
			WithArg("names", `["find-admin"]`),
			WithArg("user", "find"),
		).
			AssertSuccess(t).
			AssertEvent(t, otu.identifier("FIND", "Register"), map[string]interface{}{
				"name": "find-admin",
			})
	})

	otu.renewUserWithName("user1", "user1").
		createUser(100.0, "user2").
		registerUser("user2")

	t.Run("Should be able to send lease to another name", func(t *testing.T) {
		otu.O.Tx("moveNameTO",
			WithSigner("user1"),
			WithArg("name", "user1"),
			WithArg("receiver", "user2"),
		).
			AssertSuccess(t).
			AssertEvent(t, otu.identifier("FIND", "Moved"), map[string]interface{}{
				"name": "user1",
			})
	})

	t.Run("Should automatically set Find name to empty if sender have none", func(t *testing.T) {
		otu.O.Script("getName",
			WithArg("address", "user1"),
		).
			AssertWant(t, autogold.Want("getName empty", nil))

		otu.moveNameTo("user2", "user1", "user1")
	})

	t.Run("Should automatically set Find Name if sender have one", func(t *testing.T) {
		otu.registerUserWithName("user1", "name1").
			moveNameTo("user1", "user2", "user1")

		otu.O.Script("getName",
			WithArg("address", "user1"),
		).
			AssertWant(t, autogold.Want("getName empty", "name1"))

		otu.moveNameTo("user2", "user1", "user1")
	})

	otu.setProfile("user2")

	t.Run("Should be able to register related account and remove it", func(t *testing.T) {
		otu.O.Tx("setRelatedAccount",
			WithSigner("user1"),
			WithArg("name", "dapper"),
			WithArg("target", "user2"),
		).
			AssertSuccess(t).
			AssertEvent(t, otu.identifier("FindRelatedAccounts", "RelatedAccount"), map[string]interface{}{
				"walletName": "dapper",
				"user":       otu.O.Address("user1"),
				"address":    otu.O.Address("user2"),
				"action":     "add",
			})

		status := otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		)

		status.Print()

		status.AssertWithPointerWant(t, "/accounts/0",
			autogold.Want("getFindStatus Dapper", map[string]interface{}{
				"address": otu.O.Address("user2"),
				"name":    "dapper",
				"network": "Flow",
				"node":    "FindRelatedAccounts",
				"trusted": false,
			}))

		otu.O.Tx("removeRelatedAccount",
			WithSigner("user1"),
			WithArg("name", "dapper"),
			WithArg("network", "Flow"),
			WithArg("address", otu.O.Address("user2")),
		).
			AssertSuccess(t).
			AssertEvent(t, otu.identifier("FindRelatedAccounts", "RelatedAccount"), map[string]interface{}{
				"walletName": "dapper",
				"user":       otu.O.Address("user1"),
				"address":    otu.O.Address("user2"),
				"action":     "remove",
			})

		otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		).
			AssertWithPointerError(t, "/accounts",
				"Object has no key 'accounts'")
	})

	t.Run("Should be able to set private mode", func(t *testing.T) {
		otu.O.Tx("setPrivateMode",
			WithSigner("user1"),
			WithArg("mode", true),
		).AssertSuccess(t)

		otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		).
			AssertWithPointerWant(t, "/privateMode",
				autogold.Want("privatemode true", true),
			)

		otu.O.Tx("setPrivateMode",
			WithSigner("user1"),
			WithArg("mode", false),
		).AssertSuccess(t)

		otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		).
			AssertWithPointerWant(t, "/privateMode",
				autogold.Want("privatemode false", false),
			)
	})

	t.Run("Should be able to getFindStatus of new user", func(t *testing.T) {
		nameAddress := otu.O.Address("user3")
		otu.O.Script("getFindStatus",
			WithArg("user", nameAddress),
		).AssertWant(t,
			autogold.Want("getFindStatus", map[string]interface{}{
				"activatedAccount": true, "hasLostAndFoundItem": false,
				"isDapper":            false,
				"privateMode":         false,
				"isReadyForNameOffer": false,
				"readyForWearables":   false,
			}),
		)
	})

	t.Run("Should be able to getFindPaths of a user", func(t *testing.T) {
		nameAddress := otu.O.Address("user1")
		otu.O.Script("getFindPaths",
			WithArg("user", nameAddress),
		).AssertWant(t,
			autogold.Want("getFindPaths", map[string]interface{}{"address": "0xf669cb8d41ce0c74", "paths": []interface{}{
				"findDandy",
				"A_179b6b1cb6755e31_FindMarketDirectOfferEscrow_SaleItemCollection_find",
				"FindPackCollection",
				"A_179b6b1cb6755e31_FindLeaseMarketDirectOfferSoft_SaleItemCollection_find",
			}}),
		)
	})

	t.Run("If a user holds an invalid find name, get status should not return it", func(t *testing.T) {
		nameAddress := otu.O.Address("user2")
		otu.moveNameTo("user2", "user1", "user2")
		otu.O.Script("getFindStatus",
			WithArg("user", nameAddress),
		).AssertWithPointerError(t,
			"/profile/findName",
			"Object has no key 'findName'",
		)
	})

	t.Run("Should be able to create and edit the social link", func(t *testing.T) {
		otu.O.Tx("editProfile",
			WithSigner("user1"),
			WithArg("name", "user1"),
			WithArg("description", "This is description"),
			WithArg("avatar", "This is avatar"),
			WithArg("tags", `["This is tag"]`),
			WithArg("allowStoringFollowers", true),
			WithArg("linkTitles", map[string]string{"CryptoTwitter": "0xBjartek", "FindTwitter": "find"}),
			WithArg("linkTypes", map[string]string{"CryptoTwitter": "Twitter", "FindTwitter": "Twitter"}),
			WithArg("linkUrls", map[string]string{"CryptoTwitter": "https://twitter.com/0xBjartek", "FindTwitter": "https://twitter.com/findonflow"}),
			WithArg("removeLinks", "[]"),
		).
			AssertSuccess(t)

		otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		).AssertWithPointerWant(t,
			"/profile/links/FindTwitter",
			autogold.Want("getFindStatus Find twitter", map[string]interface{}{
				"title": "find",
				"type":  "Twitter",
				"url":   "https://twitter.com/findonflow",
			}),
		)

		otu.O.Tx("editProfile",
			WithSigner("user1"),
			WithArg("name", "user1"),
			WithArg("description", "This is description"),
			WithArg("avatar", "This is avatar"),
			WithArg("tags", `["This is tag"]`),
			WithArg("allowStoringFollowers", true),
			WithArg("linkTitles", "{}"),
			WithArg("linkTypes", "{}"),
			WithArg("linkUrls", "{}"),
			WithArg("removeLinks", `["FindTwitter"]`),
		).
			AssertSuccess(t)

		otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		).AssertWithPointerError(t,
			"/profile/links/FindTwitter",
			"Object has no key 'FindTwitter'",
		)
	})

	t.Run("Should be able to buy addons that are on Network", func(t *testing.T) {
		user := "user1"

		otu.buyForge(user)

		/* Should not be able to buy addons with wrong balance */
		otu.O.Tx("buyAddon",
			WithSigner("user1"),
			WithArg("name", "name1"),
			WithArg("addon", "forge"),
			WithArg("maxAmount", 10.0/0.42),
		).
			AssertFailure(t, "You have not sent in enough max flow")
	})

	t.Run("Should be able to fund users without profile", func(t *testing.T) {
		user := "user1"
		otu.registerFtInRegistry()

		user3 := otu.O.Address("user3")
		otu.O.Tx("sendFT",
			WithSigner(user),
			WithArg("name", user3),
			WithArg("amount", 10.0),
			WithArg("ftAliasOrIdentifier", "Flow"),
			WithArg("tag", `""`),
			WithArg("message", `""`),
		).
			AssertSuccess(t).
			AssertEmitEventName(t, "FungibleTokenSent")
	})

	t.Run("Should be able to fund users with profile but without find name", func(t *testing.T) {
		user := "user1"
		otu.createUser(1000, "user3")

		user3 := otu.O.Address("user3")

		otu.O.Tx("sendFT",
			WithSigner(user),
			WithArg("name", user3),
			WithArg("amount", 10.0),
			WithArg("ftAliasOrIdentifier", "Flow"),
			WithArg("tag", `""`),
			WithArg("message", `""`),
		).
			AssertSuccess(t).
			AssertEmitEventName(t, "FungibleTokenSent")

		otu.O.Tx("sendFT",
			WithSigner("user3"),
			WithArg("name", user),
			WithArg("amount", 10.0),
			WithArg("ftAliasOrIdentifier", "Flow"),
			WithArg("tag", `""`),
			WithArg("message", `""`),
		).
			AssertSuccess(t).
			AssertEmitEventName(t, "FungibleTokenSent")
	})

	t.Run("Should be able to fund users without profile wallet, but with vault proper set up", func(t *testing.T) {
		user := "user1"

		user3 := otu.O.Address("user3")
		otu.removeProfileWallet("user3")

		otu.O.Tx("sendFT",
			WithSigner(user),
			WithArg("name", user3),
			WithArg("amount", 10.0),
			WithArg("ftAliasOrIdentifier", "Flow"),
			WithArg("tag", `""`),
			WithArg("message", `""`),
		).
			AssertSuccess(t).
			AssertEmitEventName(t, "FungibleTokenSent")

		otu.O.Tx("sendFT",
			WithSigner("user3"),
			WithArg("name", user),
			WithArg("amount", 10.0),
			WithArg("ftAliasOrIdentifier", "Flow"),
			WithArg("tag", `""`),
			WithArg("message", `""`),
		).
			AssertSuccess(t).
			AssertEmitEventName(t, "FungibleTokenSent")
	})

	t.Run("Should be able to resolve find name without .find", func(t *testing.T) {
		otu.O.Script("resolve",
			WithArg("name", "user1.find"),
		).
			AssertWant(t, autogold.Want("user 1 address", otu.O.Address("user1")))
	})

	t.Run("Should panic if user pass in invalid character '.'", func(t *testing.T) {
		_, err := otu.O.Script("resolve",
			WithArg("name", "user1.fn"),
		).
			GetAsJson()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid byte in hex string")
	})

	t.Run("Should be able to getFindStatus of an FREE lease", func(t *testing.T) {
		res := otu.O.Script("getNameSearchbar",
			WithArg("name", "lease"),
		).
			AssertWant(t, autogold.Want("getNameSearchbar, FREE", map[string]interface{}{"cost": 5, "status": "FREE"}))

		assert.NoError(t, res.Err)
	})

	t.Run("Should be able to getFindStatus of an TAKEN lease", func(t *testing.T) {
		otu.registerUserWithName("user1", "lease")
		res := otu.O.Script("getNameSearchbar",
			WithArg("name", "lease"),
		).
			AssertWant(t, autogold.Want("getNameSearchbar, TAKEN", map[string]interface{}{
				"cost":           5,
				"lockedUntil":    1.33920005e+08,
				"owner":          "0xf669cb8d41ce0c74",
				"avatar":         "This is avatar",
				"registeredTime": 9.4608005e+07,
				"status":         "TAKEN",
				"validUntil":     1.26144005e+08,
			}))
		assert.NoError(t, res.Err)
	})

	t.Run("Should be able to getFindStatus of an LOCKED lease", func(t *testing.T) {
		otu.expireLease()
		res := otu.O.Script("getNameSearchbar",
			WithArg("name", "lease"),
		).
			Print().
			AssertWant(t, autogold.Want("getFindStatus, LOCKED", map[string]interface{}{
				"avatar":         "This is avatar",
				"cost":           5,
				"lockedUntil":    1.33920005e+08,
				"owner":          "0xf669cb8d41ce0c74",
				"registeredTime": 9.4608005e+07,
				"status":         "LOCKED",
				"validUntil":     1.26144005e+08,
			}))
		assert.NoError(t, res.Err)
	})

	user1Address := otu.O.Address("user1")
	user2Address := otu.O.Address("user2")
	user3Address := otu.O.Address("user3")
	otu.expireLease().tickClock(2.0)
	otu.registerUser("user1")

	t.Run("Should be able to register related account and mutually link it for trust", func(t *testing.T) {
		otu.O.Tx("setRelatedAccount",
			WithSigner("user1"),
			WithArg("name", "link"),
			WithArg("target", user2Address),
		).
			AssertSuccess(t)

		otu.O.Script("devgetLinked",
			WithArg("user", "user1"),
			WithArg("name", "link"),
			WithArg("address", user2Address),
		).
			AssertWant(t, autogold.Want("should be false, not linked", false))

		otu.O.Tx("setRelatedAccount",
			WithSigner("user2"),
			WithArg("name", "wrongName"),
			WithArg("target", user1Address),
		).
			AssertSuccess(t)

		otu.O.Script("devgetLinked",
			WithArg("user", "user1"),
			WithArg("name", "link"),
			WithArg("address", user2Address),
		).
			AssertWant(t, autogold.Want("should be false, wrong name", false))

		otu.O.Tx("setRelatedAccount",
			WithSigner("user2"),
			WithArg("name", "link"),
			WithArg("target", user1Address),
		).
			AssertSuccess(t)

		otu.O.Script("devgetLinked",
			WithArg("user", "user1"),
			WithArg("name", "link"),
			WithArg("address", user2Address),
		).
			AssertWant(t, autogold.Want("should be true", true))

		otu.O.Tx("removeRelatedAccount",
			WithSigner("user2"),
			WithArg("name", "link"),
			WithArg("network", "Flow"),
			WithArg("address", user1Address),
		).
			AssertSuccess(t)

		otu.O.Script("devgetLinked",
			WithArg("user", "user1"),
			WithArg("name", "link"),
			WithArg("address", user2Address),
		).
			AssertWant(t, autogold.Want("should be false, removed link", false))

		otu.O.Tx("removeRelatedAccount",
			WithSigner("user1"),
			WithArg("name", "link"),
			WithArg("network", "Flow"),
			WithArg("address", user2Address),
		).
			AssertSuccess(t)

		otu.O.Tx("removeRelatedAccount",
			WithSigner("user2"),
			WithArg("name", "wrongName"),
			WithArg("network", "Flow"),
			WithArg("address", user1Address),
		).
			AssertSuccess(t)
	})

	t.Run("Should be able to getFindStatus for trusted accounts", func(t *testing.T) {
		otu.O.Tx("setRelatedAccount",
			WithSigner("user1"),
			WithArg("name", "link"),
			WithArg("target", user2Address),
		).
			AssertSuccess(t)

		otu.O.Tx("setRelatedAccount",
			WithSigner("user1"),
			WithArg("name", "notLink"),
			WithArg("target", user3Address),
		).
			AssertSuccess(t)

		otu.O.Tx("setRelatedAccount",
			WithSigner("user2"),
			WithArg("name", "link"),
			WithArg("target", user1Address),
		).
			AssertSuccess(t)

		otu.O.Script("getFindStatus",
			WithArg("user", "user1"),
		).
			Print().
			AssertWithPointerWant(t, "/accounts",
				autogold.Want("with accounts", `[]interface {}{
  map[string]interface {}{
    "address": "0xfd43f9148d4b725d",
    "name": "notLink",
    "network": "Flow",
    "node": "FindRelatedAccounts",
    "trusted": false,
  },
  map[string]interface {}{
    "address": "0x192440c99cb17282",
    "name": "link",
    "network": "Flow",
    "node": "FindRelatedAccounts",
    "trusted": true,
  },
}`))
	})
	otu.registerUser("user2")
	t.Run("Should be able to follow someone", func(t *testing.T) {
		otu.O.Tx(
			"follow",
			WithSigner("user1"),
			WithArg("follows", map[string][]string{
				"user2": {"user2", "man"},
			}),
		).
			AssertSuccess(t).
			AssertEvent(t, "Follow", map[string]interface{}{
				"follower":  otu.O.Address("user1"),
				"following": otu.O.Address("user2"),
				"tags":      []interface{}{"user2", "man"},
			})
	})

	t.Run("Should be able to unfollow someone", func(t *testing.T) {
		otu.O.Tx(
			"unfollow",
			WithSigner("user1"),
			WithArg("unfollows", []string{
				"user2",
			}),
		).
			AssertSuccess(t).
			AssertEvent(t, "Unfollow", map[string]interface{}{
				"follower":    otu.O.Address("user1"),
				"unfollowing": otu.O.Address("user2"),
			})
	})

	oldOwner := "user1"
	currentOwner := "user2"
	testingName := "testingname"

	// setup for testing old leases
	otu.registerUserWithName(oldOwner, testingName)

	otu.expireLease().
		expireLease().
		tickClock(2.0)

	otu.registerUserWithName(currentOwner, testingName)

	t.Run("Should not be able to get old leases information", func(t *testing.T) {
		otu.O.Script(`
			import FIND from "../contracts/FIND.cdc"

			pub fun main(user: Address) : [FIND.LeaseInformation] {
			let finLeases= getAuthAccount(user).borrow<&FIND.LeaseCollection>(from:FIND.LeaseStoragePath)!
			return finLeases.getLeaseInformation()
			}
		`,
			WithArg("user", oldOwner),
		).
			AssertWant(t, autogold.Want("should be nil", nil))
	})

	t.Run("Should not be able to get old leases", func(t *testing.T) {
		otu.O.Script(`
			import FIND from "../contracts/FIND.cdc"

			pub fun main(user: Address) : [String] {
			let finLeases= getAuthAccount(user).borrow<&FIND.LeaseCollection>(from:FIND.LeaseStoragePath)!
			return finLeases.getLeases()
			}
		`,
			WithArg("user", oldOwner),
		).
			AssertWant(t, autogold.Want("should be nil for lease name", `[]interface {}{
  "user1",
  "lease",
  "name1",
}`))
	})

	t.Run("Should be able to get old leases in getInvalidatedLeases", func(t *testing.T) {
		otu.O.Script(`
			import FIND from "../contracts/FIND.cdc"

			pub fun main(user: Address) : [String] {
			let finLeases= getAuthAccount(user).borrow<&FIND.LeaseCollection>(from:FIND.LeaseStoragePath)!
			return finLeases.getInvalidatedLeases()
			}
		`,
			WithArg("user", oldOwner),
		).
			AssertWant(t, autogold.Want("should not be nil for lease name", `[]interface {}{
  "user2",
  "testingname",
}`))
	})

	t.Run("Should be able to cleanup invalid leases", func(t *testing.T) {
		otu.O.Tx("cleanUpInvalidatedLease",
			WithSigner(currentOwner),
			WithArg("names", []string{testingName}),
		).
			AssertFailure(t, "This is a valid lease. You cannot clean this up. Lease : testingname")

		otu.O.Tx("cleanUpInvalidatedLease",
			WithSigner(oldOwner),
			WithArg("names", []string{testingName}),
		).AssertSuccess(t)
	})
}
