import FIND from "../contracts/FIND.cdc"

transaction(owner: Address, name: String) {
	prepare(account: AuthAccount) {

		let leaseCollection = getAccount(owner).getCapability<&{FIND.LeaseCollectionPublic}>(FIND.LeasePublicPath)
		leaseCollection.borrow()!.fullfillAuction(name)

	}
}
