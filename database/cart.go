package database

import (
	"context"
	"errors"
	"go-ecommerce/models"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrCantFindProduct = errors.New("can't find the product")
	ErrCantDecodeProducts = errors.New("can't find the product")
	ErrUserIdIsNotValid = errors.New("this user is not valid")
	ErrCantUpdateUser = errors.New("cannot add this product to the cart")
	ErrCantRemoveItemCart = errors.New("cannot remove this item from the cart")
	ErrCantGetItems = errors.New("was unable to get the item from the cart")
	ErrCantBuyCartItems = errors.New("cannot update the purchase")
)

func AddProductToCart(ctx context.Context, prodCollection, userCollection *mongo.Collection, productID primitive.ObjectID, userID string) error {
	searchFromDB, err := prodCollection.Find(ctx, bson.M{"_id": productID})
	if err != nil {
		log.Println(err)
		return ErrCantFindProduct
	}
	var productCart []models.ProductUser
	err = searchFromDB.All(ctx, &productCart)
	if err != nil {
		log.Println(err)
		return ErrCantDecodeProducts
	}
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Println(err)
		return ErrUserIdIsNotValid
	}
	filtered := bson.D{primitive.E{Key:"_id", Value: id}}
	update := bson.D{primitive.E{Key:"$push", Value: bson.D{primitive.E{Key: "usercart", Value: bson.D{{Key:"$each", Value: productCart}}}}}}
	_, err = userCollection.UpdateOne(ctx, filtered, update)
	if err != nil {
		return ErrCantUpdateUser
	}
	return nil
}

func RemoveCartItem(ctx context.Context, prodCollection, userCollection *mongo.Collection, productID primitive.ObjectID, userID string) error {
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Println(err)
		return ErrUserIdIsNotValid
	}
	filtered := bson.D{primitive.E{Key:"_id", Value: id}}
	update := bson.M{"$pull":bson.M{"usercart": bson.M{"_id": productID}}}
	_, err = userCollection.UpdateMany(ctx, filtered, update)
	if err != nil {
		return ErrCantRemoveItemCart
	}
	return nil
}

func BuyItemFromCart(ctx context.Context, userCollection *mongo.Collection, userID string) error {
	//fetch the cart of the user
	id, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Println(err)
		return ErrUserIdIsNotValid
	}
	var getCartitems models.User
	var orderCart models.Order
	orderCart.Order_ID = primitive.NewObjectID()
	orderCart.Ordered_At = time.Now()
	orderCart.Order_Cart = make([]models.ProductUser, 0)
	orderCart.Payment_Method.COD = true

	//find the cart total
	unwind := bson.D{{Key:"$unwind", Value: bson.D{primitive.E{Key:"path", Value:"$usercart"}}}}
	grouping := bson.D{{Key:"$group", Value:bson.D{primitive.E{Key:"_id", Value:"$_id"}, {Key:"total", Value: bson.D{primitive.E{Key:"$sum", Value:"$usercart.price"}}}}}}

	currentResults, err := userCollection.Aggregate(ctx, mongo.Pipeline{unwind, grouping})

	ctx.Done()
	if err != nil {
		panic(err)
	}

	var getUserCart []bson.M
	if err = currentResults.All(ctx, &getUserCart); err != nil {
		panic(err)
	}

	var totalPrice int32

	for _, userItem := range getUserCart{
		price := userItem["total"]
		totalPrice = price.(int32)
	}

	orderCart.Price = int(totalPrice)

	//create an order with the items
	
	//added order to the user Collection
	filter := bson.D{primitive.E{Key:"_id", Value: id}}
	update := bson.D{{Key:"$push", Value:bson.D{primitive.E{Key:"orders", Value: orderCart}}}}
	_, err = userCollection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Println(err)
	}

	err = userCollection.FindOne(ctx, bson.D{primitive.E{Key:"_id", Value: id}}).Decode(&getCartitems)
	if err != nil {
		log.Println(err)
	}

	
	//added items in the cart to order list
	filter2 := bson.D{primitive.E{Key:"_id", Value:id}}
	update2 := bson.M{"$push":bson.M{"orders.$[].order_list": bson.M{"$each": getCartitems.UserCart}}}
	userCollection.UpdateOne(ctx, filter2, update2)
	if err != nil {
		log.Println(err)
	}

	//empty up the cart
	userCartEmpty := make([]models.ProductUser, 0)
	filter3 := bson.D{primitive.E{Key:"_id", Value: id}}
	update3 := bson.D{{Key:"$set", Value:bson.D{primitive.E{Key:"usercart", Value: userCartEmpty}}}}

	_, err = userCollection.UpdateOne(ctx, filter3, update3)

	if err != nil {
		log.Println(err)
	}

	return nil
}

func InstantBuyer(ctx context.Context, prodCollection, userCollection *mongo.Collection, productID primitive.ObjectID, userID string) error {
	id, err := primitive.ObjectIDFromHex(userID)

	if err != nil {
		log.Println(err)
		return ErrUserIdIsNotValid
	}

	var productDetails models.ProductUser
	var orderDetails models.Order

	orderDetails.Order_ID = primitive.NewObjectID()
	orderDetails.Ordered_At = time.Now()
	orderDetails.Order_Cart = make([]models.ProductUser, 0)
	orderDetails.Payment_Method.COD = true
	err = prodCollection.FindOne(ctx, bson.D{primitive.E{Key:"_id", Value: productID}}).Decode(&productDetails)
	if err != nil {
		log.Println(err)
	}
	orderDetails.Price = productDetails.Price

	filter := bson.D{primitive.E{Key:"_id", Value: id}}
	update := bson.D{{Key: "$push", Value: bson.D{primitive.E{Key:"orders", Value:orderDetails}}}}

	_, err = userCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Println(err)
	}

	filter2 := bson.D{primitive.E{Key:"_id", Value: id}}
	update2 := bson.M{"$push": bson.M{"orders.$[].order_list": productDetails}}

	_, err = userCollection.UpdateOne(ctx, filter2, update2)
	if err != nil {
		log.Println(err)
	}

	return nil
}